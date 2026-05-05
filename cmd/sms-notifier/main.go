package main

import (
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
	port        string
	accountSID  string
	authToken   string
	from        string
	to          string
	dryRun      bool
	callbackURL string
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

	logger.Info("sms notifier listening", "addr", server.Addr, "dry_run", cfg.dryRun)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("sms notifier stopped", "error", err)
		os.Exit(1)
	}
}

func loadConfig() config {
	return config{
		port:        env("PORT", "8090"),
		accountSID:  os.Getenv("TWILIO_ACCOUNT_SID"),
		authToken:   os.Getenv("TWILIO_AUTH_TOKEN"),
		from:        os.Getenv("TWILIO_FROM"),
		to:          os.Getenv("SMS_TO"),
		dryRun:      strings.EqualFold(env("SMS_DRY_RUN", "false"), "true"),
		callbackURL: env("TWILIO_API_BASE_URL", "https://api.twilio.com"),
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

	message := smsMessage(payload)
	if cfg.dryRun {
		logger.Info("sms dry run", "to", cfg.to, "message", message)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "dry_run"})
		return
	}

	if err := cfg.validate(); err != nil {
		logger.Error("sms notifier is not configured", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := sendTwilioSMS(r.Context(), cfg, message); err != nil {
		logger.Error("failed to send sms", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send sms"})
		return
	}

	logger.Info("sms sent", "to", cfg.to, "alert_status", payload.Status)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

func (cfg config) validate() error {
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

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", strings.TrimRight(cfg.callbackURL, "/"), cfg.accountSID)
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

func smsMessage(payload grafanaWebhook) string {
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
