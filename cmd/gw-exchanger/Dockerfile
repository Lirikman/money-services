# Stage 1: Build binary
FROM alpine:latest AS healthcheck

ARG GRPC_HEALTH_PROBE_VERSION=v0.4.40

RUN wget -O /grpc_health_probe \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
    && chmod +x /grpc_health_probe

FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -o exchanger-bin ./cmd/gw-exchanger/main.go

# Stage 2: Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/exchanger-bin ./main
COPY --from=builder /app/services/gw-exchanger/migrations ./services/gw-exchanger/migrations
COPY --from=healthcheck /grpc_health_probe /bin/grpc_health_probe

EXPOSE 50051

CMD ["./main", "-c", "config.env"]