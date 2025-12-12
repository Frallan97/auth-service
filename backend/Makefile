.PHONY: help build run test clean migrate-up migrate-down migrate-create keys

help:
	@echo "Available commands:"
	@echo "  make build        - Build the application"
	@echo "  make run          - Run the application"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make migrate-up   - Run database migrations"
	@echo "  make migrate-down - Rollback database migrations"
	@echo "  make migrate-create NAME=<name> - Create new migration"
	@echo "  make keys         - Generate RSA keys for JWT"

build:
	go build -o bin/auth-service ./cmd/api

run:
	go run ./cmd/api/main.go

test:
	go test -v ./...

clean:
	rm -rf bin/
	go clean

migrate-up:
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Error: DATABASE_URL not set. Run: export DATABASE_URL='postgresql://...'"; \
		exit 1; \
	fi
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Error: DATABASE_URL not set. Run: export DATABASE_URL='postgresql://...'"; \
		exit 1; \
	fi
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME not set. Run: make migrate-create NAME=migration_name"; \
		exit 1; \
	fi
	migrate create -ext sql -dir migrations -seq $(NAME)

keys:
	@mkdir -p keys
	@openssl genrsa -out keys/private_key.pem 4096
	@openssl rsa -in keys/private_key.pem -pubout -out keys/public_key.pem
	@echo "RSA keys generated in keys/ directory"
