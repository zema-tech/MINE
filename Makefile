MINE_VERSION ?= dev
PANEL_IMAGE ?= ghcr.io/zema-tech/mine-panel:$(MINE_VERSION)
DAEMON_IMAGE ?= ghcr.io/zema-tech/mine-daemon:$(MINE_VERSION)

.PHONY: help build test lint migrate up down generate clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build panel + daemon binaries
	cd panel/backend && go build -o bin/mine ./cmd/mine
	cd daemon && go build -o bin/mined ./cmd/mined
	cd shared && go build ./...

build-docker: ## Build docker images
	docker build -t $(PANEL_IMAGE) -f panel/backend/Dockerfile panel/backend
	docker build -t ghcr.io/zema-tech/mine-frontend:$(MINE_VERSION) -f panel/frontend/Dockerfile panel/frontend
	docker build -t $(DAEMON_IMAGE) -f daemon/Dockerfile daemon

test: ## Run all Go tests
	cd panel/backend && go test ./... -race -count=1
	cd daemon && go test ./... -race -count=1
	cd shared && go test ./... -count=1

lint: ## Lint Go code
	cd panel/backend && go vet ./...
	cd daemon && go vet ./...
	cd shared && go vet ./...

migrate-up: ## Apply migrations (requires DATABASE_URL)
	cd panel/backend && migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Rollback last migration
	cd panel/backend && migrate -path migrations -database "$(DATABASE_URL)" down

migrate-new: ## Create new migration NAME=xxx
	cd panel/backend && migrate create -ext sql -dir migrations -seq $(NAME)

up: ## Start full dev stack
	docker compose up --build

up-prod: ## Start production stack
	docker compose -f docker-compose.prod.yml up -d --build

down: ## Stop stack
	docker compose down -v

generate: ## Regenerate TLS / secrets for dev
	./scripts/setup-dev.sh

clean: ## Remove build artifacts
	rm -rf panel/backend/bin daemon/bin shared/bin panel/frontend/dist
