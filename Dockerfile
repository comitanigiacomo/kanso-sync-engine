# --- Stage 1: Builder ---
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o kanso-api ./cmd/api

# --- Stage 2: Runner ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/kanso-api .

EXPOSE 8080

CMD ["./kanso-api"]