# Stage 1: Build binary
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
COPY --from=builder /app/cmd/gw-exchanger/config.env .

EXPOSE 50051

CMD ["./main", "-c", "config.env"]