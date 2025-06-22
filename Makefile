.PHONY: help install-tools migrate-up migrate-down migrate-create sqlc-generate setup build build-linux run run-help dev test test-verbose test-coverage lint check clean db-reset db-debug deps install-dev-tools

# Default target
.DEFAULT_GOAL := help

# Help command - automatically generate help from comments
help: ## Show this help message
	@echo 'Usage:'
	@echo '  make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Bot Usage:'
	@echo '  ./discord-activity-bot --help'

install-tools: ## Install required development tools (migrate, sqlc)
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

migrate-up: ## Run database migrations up
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Run database migrations down
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create: ## Create a new migration file
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

sqlc-generate: ## Generate Go code from SQL queries
	sqlc generate -f configs/sqlc.yaml

setup: migrate-up sqlc-generate ## Run migrations and generate sqlc code

# Version variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS = -ldflags="-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT) -s -w"

build: ## Build the bot binary (using Go)
	go build $(LDFLAGS) -o discord-activity-bot ./cmd/discord-activity-bot

build-linux: ## Build for Linux (useful for Docker)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o discord-activity-bot-linux ./cmd/discord-activity-bot

build-bazel: ## Build the bot binary using Bazel (recommended)
	bazelisk build //cmd/discord-activity-bot:discord-activity-bot --stamp
	cp bazel-bin/cmd/discord-activity-bot/discord-activity-bot_/discord-activity-bot discord-activity-bot

build-bazel-linux: ## Build for Linux using Bazel
	bazelisk build //cmd/discord-activity-bot:discord-activity-bot --stamp --platforms=@rules_go//go/toolchain:linux_amd64
	cp bazel-bin/cmd/discord-activity-bot/discord-activity-bot_/discord-activity-bot discord-activity-bot-linux

version: ## Show version information that would be built
	@echo "Version: $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Git Commit: $(GIT_COMMIT)"

run: ## Run the bot
	go run ./cmd/discord-activity-bot

run-help: ## Show bot help/usage
	go run ./cmd/discord-activity-bot --help

dev: ## Run with auto-restart on changes (requires entr)
	find . -name "*.go" | entr -r go run ./cmd/discord-activity-bot

test: ## Run tests (using Go)
	go test ./...

test-verbose: ## Run tests with verbose output
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-bazel: ## Run tests using Bazel (recommended)
	bazelisk test //...

lint: ## Run linter
	golangci-lint run

check: lint test ## Run all checks (lint and test)

clean: ## Clean build artifacts
	rm -f discord-activity-bot discord-activity-bot-linux discord-bot
	rm -f coverage.out coverage.html

db-reset: ## Drop and recreate the database (WARNING: destroys all data!)
	@echo "WARNING: This will destroy all data in the discord_activity database!"
	@read -p "Are you sure? [y/N] " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		if [ -n "$(DATABASE_URL)" ]; then \
			psql "$$(echo $(DATABASE_URL) | sed 's|/discord_activity.*|/postgres|')" -f dev/reset_db.sql && \
			echo "Database reset complete."; \
		else \
			psql -f dev/reset_db.sql && \
			echo "Database reset complete."; \
		fi \
	else \
		echo "Database reset cancelled."; \
	fi

db-debug: ## Run debug queries to check database state
	@if [ -n "$(DATABASE_URL)" ]; then \
		psql "$(DATABASE_URL)" -f dev/debug.sql; \
	else \
		psql discord_activity -f dev/debug.sql; \
	fi

deps: ## Download and tidy dependencies
	go mod download
	go mod tidy


install-dev-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/evilmartians/lefthook@latest
