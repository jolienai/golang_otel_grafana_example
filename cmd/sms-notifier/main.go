package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type config struct {
	port                    string
	accountSID              string
	authToken               string
	from                    string
	to                      string
	smsDryRun               bool
	twilioAPIBaseURL        string
	pagerDutyIntegrationKey string
	pagerDutyDryRun         bool
	pagerDutyEventsURL      string
}

type grafanaWebhook struct {
	Status  string         `json:"status"`
	Title   string         `json:"title"`
	Message string         `json:"message"`
	Alerts  []grafanaAlert `json:"alerts"`
}

type grafanaAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	ValueString string            `json:"valueString"`
}

type pagerDutyEvent struct {
	RoutingKey    string                `json:"routing_key"`
	EventAction   string                `json:"event_action"`
	DedupKey      string                `json:"dedup_key,omitempty"`
	Payload       pagerDutyEventPayload `json:"payload"`
	CustomDetails map[string]string     `json:"custom_details,omitempty"`
}

type pagerDutyEventPayload struct {
	Summary   string `json:"summary"`
	Source    string `json:"source"`
	Severity  string `json:"severity"`
	Component string `json:"component,omitempty"`
	Group     string `json:"group,omitempty"`
	Class     string `json:"class,omitempty"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := loadConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /grafana-alert", func(w http.ResponseWriter, r *http.Request) {
		handleGrafanaAlert(w, r, cfg, logger)
	})

	server := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("alert notifier listening", "addr", server.Addr, "sms_dry_run", cfg.smsDryRun, "pagerduty_dry_run", cfg.pagerDutyDryRun)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("sms notifier stopped", "error", err)
		os.Exit(1)
	}
}

func loadConfig() config {
	return config{
		port:                    env("PORT", "8090"),
		accountSID:              os.Getenv("TWILIO_ACCOUNT_SID"),
		authToken:               os.Getenv("TWILIO_AUTH_TOKEN"),
		from:                    os.Getenv("TWILIO_FROM"),
		to:                      os.Getenv("SMS_TO"),
		smsDryRun:               strings.EqualFold(env("SMS_DRY_RUN", "false"), "true"),
		twilioAPIBaseURL:        env("TWILIO_API_BASE_URL", "https://api.twilio.com"),
		pagerDutyIntegrationKey: os.Getenv("PAGERDUTY_INTEGRATION_KEY"),
		pagerDutyDryRun:         strings.EqualFold(env("PAGERDUTY_DRY_RUN", "true"), "true"),
		pagerDutyEventsURL:      env("PAGERDUTY_EVENTS_URL", "https://events.pagerduty.com/v2/enqueue"),
	}
}

func handleGrafanaAlert(w http.ResponseWriter, r *http.Request, cfg config, logger *slog.Logger) {
	defer r.Body.Close()

	var payload grafanaWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Warn("invalid grafana webhook payload", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}

	message := alertMessage(payload)
	deliveryErrors := make([]error, 0)

	if err := notifySMS(r.Context(), cfg, message, payload, logger); err != nil {
		deliveryErrors = append(deliveryErrors, err)
	}
	if err := notifyPagerDuty(r.Context(), cfg, payload, logger); err != nil {
		deliveryErrors = append(deliveryErrors, err)
	}
	if len(deliveryErrors) > 0 {
		logger.Error("failed to send one or more alert notifications", "error", errors.Join(deliveryErrors...))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send one or more alert notifications"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func notifySMS(ctx context.Context, cfg config, message string, payload grafanaWebhook, logger *slog.Logger) error {
	if cfg.smsDryRun {
		logger.Info("sms dry run", "to", cfg.to, "message", message)
		return nil
	}

	if err := cfg.validateTwilio(); err != nil {
		logger.Error("sms notifier is not configured", "error", err)
		return err
	}

	if err := sendTwilioSMS(ctx, cfg, message); err != nil {
		logger.Error("failed to send sms", "error", err)
		return err
	}

	logger.Info("sms sent", "to", cfg.to, "alert_status", payload.Status)
	return nil
}

func notifyPagerDuty(ctx context.Context, cfg config, payload grafanaWebhook, logger *slog.Logger) error {
	event := pagerDutyEventFromGrafana(payload, cfg.pagerDutyIntegrationKey)
	if cfg.pagerDutyDryRun {
		logger.Info("pagerduty dry run", "event_action", event.EventAction, "dedup_key", event.DedupKey, "summary", event.Payload.Summary)
		return nil
	}

	if cfg.pagerDutyIntegrationKey == "" {
		err := errors.New("missing required env var: PAGERDUTY_INTEGRATION_KEY")
		logger.Error("pagerduty notifier is not configured", "error", err)
		return err
	}

	if err := sendPagerDutyEvent(ctx, cfg, event); err != nil {
		logger.Error("failed to send pagerduty event", "error", err)
		return err
	}

	logger.Info("pagerduty event sent", "event_action", event.EventAction, "dedup_key", event.DedupKey)
	return nil
}

func (cfg config) validateTwilio() error {
	missing := make([]string, 0)
	for name, value := range map[string]string{
		"TWILIO_ACCOUNT_SID": cfg.accountSID,
		"TWILIO_AUTH_TOKEN":  cfg.authToken,
		"TWILIO_FROM":        cfg.from,
		"SMS_TO":             cfg.to,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func sendTwilioSMS(ctx context.Context, cfg config, message string) error {
	form := url.Values{}
	form.Set("To", cfg.to)
	form.Set("From", cfg.from)
	form.Set("Body", message)

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", strings.TrimRight(cfg.twilioAPIBaseURL, "/"), cfg.accountSID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.accountSID, cfg.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("twilio returned status %d", resp.StatusCode)
	}
	return nil
}

func sendPagerDutyEvent(ctx context.Context, cfg config, event pagerDutyEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.pagerDutyEventsURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pagerduty returned status %d", resp.StatusCode)
	}
	return nil
}

func alertMessage(payload grafanaWebhook) string {
	title := payload.Title
	if title == "" {
		title = "Grafana alert"
	}

	status := strings.ToUpper(payload.Status)
	if status == "" {
		status = "UNKNOWN"
	}

	summary := payload.Message
	for _, alert := range payload.Alerts {
		if value := alert.Annotations["summary"]; value != "" {
			summary = value
			break
		}
	}
	if summary == "" {
		summary = "Check Grafana for details."
	}

	return trimSMS(fmt.Sprintf("[%s] %s: %s", status, title, summary))
}

func pagerDutyEventFromGrafana(payload grafanaWebhook, routingKey string) pagerDutyEvent {
	title := payload.Title
	if title == "" {
		title = "Grafana alert"
	}

	status := strings.ToLower(payload.Status)
	action := "trigger"
	if status == "resolved" {
		action = "resolve"
	}

	summary := title
	dedupKey := "grafana-alert"
	severity := "warning"
	for _, alert := range payload.Alerts {
		if value := alert.Annotations["summary"]; value != "" {
			summary = value
		}
		if value := alert.Labels["alertname"]; value != "" {
			dedupKey = value
		}
		if value := alert.Labels["severity"]; value != "" {
			severity = pagerDutySeverity(value)
		}
		break
	}

	return pagerDutyEvent{
		RoutingKey:  routingKey,
		EventAction: action,
		DedupKey:    dedupKey,
		Payload: pagerDutyEventPayload{
			Summary:   summary,
			Source:    "grafana",
			Severity:  severity,
			Component: "golang-otel-api",
			Group:     "docker-compose",
			Class:     "api-alert",
		},
		CustomDetails: map[string]string{
			"grafana_status": payload.Status,
			"grafana_title":  title,
		},
	}
}

func pagerDutySeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "error", "warning", "info":
		return strings.ToLower(severity)
	default:
		return "warning"
	}
}

func trimSMS(message string) string {
	const maxLength = 320
	if len(message) <= maxLength {
		return message
	}
	return message[:maxLength-3] + "..."
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
