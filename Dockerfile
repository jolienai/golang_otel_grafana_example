FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./src

FROM alpine:3.20

RUN adduser -D -H appuser
USER appuser

COPY --from=build /bin/api /api

EXPOSE 8080
ENTRYPOINT ["/api"]
