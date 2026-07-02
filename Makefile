.PHONY: run build tidy test vet fmt

# Run the backend against a local .env (export it first, e.g. `set -a; . ./.env; set +a`).
run:
	cd backend && go run ./cmd/mailfold

build:
	cd backend && go build -o ../bin/mailfold ./cmd/mailfold

tidy:
	cd backend && go mod tidy

test:
	cd backend && go test ./...

vet:
	cd backend && go vet ./...

fmt:
	cd backend && gofmt -w .
