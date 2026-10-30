# syntax=docker/dockerfile:1

# --- Сборка ---
FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/service .

# --- Runtime ---
FROM alpine:latest
WORKDIR /app

RUN addgroup -S app && adduser -S -G app app
RUN mkdir -p /data && chown -R app:app /data

COPY --from=builder /app/bin/service ./service

USER app
EXPOSE 3000
CMD ["./service"]
