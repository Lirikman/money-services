# Stage 1: Build binary
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -o currency-wallet-bin ./cmd/gw-currency-wallet/main.go

# Stage 2: Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/currency-wallet-bin ./main
COPY --from=builder /app/services/gw-currency-wallet/migrations ./services/gw-currency-wallet/migrations

EXPOSE 8080

CMD ["./main", "-c", "config.env"]