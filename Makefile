SHELL := /usr/bin/env bash
BINARY_NAME := sdui-bff
BIN_DIR := bin
CMD_PKG := ./cmd/sdui-bff
GO ?= go

.PHONY: help build run run-with-layout-engine test test-race cover vet fmt fmt-check hygiene verify tidy docker-build docker-up docker-down clean

help: ## Show available Make targets
	@awk 'BEGIN {FS = ":.*##"; printf "\nAbeta SDUI Domain BFF — Make Targets:\n\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2 } END { printf "\n" }' $(MAKEFILE_LIST)

build: ## Build a static local binary into bin/sdui-bff
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PKG)

run: ## Run the SDUI BFF standalone on :8085 (zero external dependencies)
	$(GO) run $(CMD_PKG)

run-with-layout-engine: ## Run the SDUI BFF on :8085 connected to layout-engine on :8090
	LAYOUT_ENGINE_URL=$${LAYOUT_ENGINE_URL:-http://localhost:8090} $(GO) run $(CMD_PKG)

test: ## Run unit and integration tests with race detector and coverage
	$(GO) test -count=1 -race -cover ./...

test-race: ## Alias for race-enabled test run with coverage
	$(GO) test -count=1 -race -cover ./...

cover: ## Generate HTML coverage report in coverage.html
	$(GO) test -count=1 -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

vet: ## Run go vet static analysis
	$(GO) vet ./...

fmt: ## Format all Go source files with gofmt
	gofmt -w .

fmt-check: ## Verify all Go source files are gofmt-clean
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted Go files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

hygiene: ## Run brand and machine-path hygiene checks
	./scripts/check_brand_hygiene.sh

verify: ## Run the full local verification pipeline
	./scripts/verify_all.sh

tidy: ## Synchronize go.mod and go.sum
	$(GO) mod tidy

docker-build: ## Build the local distroless OCI container image
	docker build -t abeta-sdui-bff:local .

docker-up: ## Start the 3-service Abeta SDUI Platform stack (Postgres + Layout Engine + SDUI BFF)
	docker compose up --build

docker-down: ## Stop Docker Compose services
	docker compose down

clean: ## Remove build outputs, local binaries, and coverage artifacts
	rm -rf $(BIN_DIR) dist sdui-bff coverage.out coverage.html
