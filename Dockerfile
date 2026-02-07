# --- Stage 1: Builder ---
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o kanso-api ./cmd/api

# --- Stage 2: Runner (Production Image) ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -S kanso && adduser -S kanso -G kanso

WORKDIR /app

COPY --from=builder /app/kanso-api .

RUN chown kanso:kanso ./kanso-api

USER kanso

EXPOSE 8080

CMD ["./kanso-api"]