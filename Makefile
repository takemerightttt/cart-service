.PHONY: run up down lint test test-unit test-integration cover build

run:
	go run ./cmd/api

up:
	docker compose up --build -d

down:
	docker compose down

build:
	CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api

lint:
	golangci-lint run ./...

test: test-unit test-integration

test-unit:
	go test -race -count=1 ./internal/...

test-integration:
	go test -race -count=1 ./tests/integration/...

cover:
	go test -race -count=1 -coverpkg=./internal/... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
