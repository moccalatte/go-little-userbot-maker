APP_NAME=go-little-userbot-maker
GO_VERSION?=1.22

.PHONY: build build-wizard build-userbot test lint compose-up-local compose-down-local run-wizard run-userbot dev-mock

build: build-wizard build-userbot

build-wizard:
	GOOS=linux GOARCH=amd64 go build -o bin/wizard ./services/bot-wizard/cmd

build-userbot:
	GOOS=linux GOARCH=amd64 go build -o bin/userbot ./services/userbot-orchestrator/cmd

test:
	go test ./...

lint:
	golangci-lint run || true

compose-up-local:
	docker compose -f docker-compose.local.yml up --build -d

compose-down-local:
	docker compose -f docker-compose.local.yml down

run-wizard:
	go run ./services/bot-wizard/cmd

run-userbot:
	go run ./services/userbot-orchestrator/cmd

dev-mock:
	./scripts/dev_mock.sh