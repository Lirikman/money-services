clean:
	rm proto-exchange/generate/*

gen:
	protoc --proto_path=proto-exchange/exchange \
		--go_out=proto-exchange/generate --go_opt=paths=source_relative \
		--go-grpc_out=proto-exchange/generate --go-grpc_opt=paths=source_relative \
		--experimental_allow_proto3_optional exchange.proto

run-svc1:
	go run cmd/gw-exchanger/main.go -c cmd/gw-exchanger/config.env

run-svc2:
	go run cmd/gw-currency-wallet/main.go -c cmd/gw-currency-wallet/config.env

run-svc3:
	go run cmd/gw-notification/main.go -c cmd/gw-notification/config.env

swag-init:
	swag init -d ./ -g cmd/gw-currency-wallet/main.go --parseInternal --parseDependency
