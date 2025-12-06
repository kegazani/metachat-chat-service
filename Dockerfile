FROM golang:1.24 AS builder

WORKDIR /app

COPY metachat-chat-service/go.mod ./
COPY metachat-chat-service/go.sum* ./

RUN go mod download

COPY metachat-chat-service/ .

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/main.go

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/main .

COPY --from=builder /app/config ./config

EXPOSE 50055

CMD ["./main"]

