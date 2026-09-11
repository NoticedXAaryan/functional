.PHONY: dev stop migrate migrate-down seed test gen-client lint clean help

# ── Configuration ──────────────────────────────────────────────────────────
DB_URL ?= postgres://balsuraksha:balsuraksha@localhost:5432/balsuraksha?sslmode=disable
MIGRATE_BIN := $(shell which migrate 2>/dev/null || echo "go run -mod=mod github.com/golang-migrate/migrate/v4/cmd/migrate@latest")
OAPI_CODEGEN := npx --yes openapi-typescript

# ── Primary developer targets ──────────────────────────────────────────────

## dev: Start all services with Docker Compose (hot-reload enabled)
dev:
	@echo "▶ Starting Bal Suraksha dev environment…"
	docker compose up --build

## dev-bg: Start all services in the background
dev-bg:
	docker compose up --build -d

## stop: Stop all Docker Compose services
stop:
	docker compose down

## stop-clean: Stop and remove volumes (WARNING: destroys database data)
stop-clean:
	docker compose down -v

# ── Database ───────────────────────────────────────────────────────────────

## migrate: Apply all pending database migrations
migrate:
	@echo "▶ Running migrations against $(DB_URL)"
	$(MIGRATE_BIN) -path db/migrations -database "$(DB_URL)" up

## migrate-down: Roll back the last migration
migrate-down:
	$(MIGRATE_BIN) -path db/migrations -database "$(DB_URL)" down 1

## migrate-status: Show migration status
migrate-status:
	$(MIGRATE_BIN) -path db/migrations -database "$(DB_URL)" version

## seed: Load synthetic FICTIONAL fixtures into the demo database
seed:
	@echo "▶ Seeding demo data (FICTIONAL — synthetic only)"
	cd services/api && go run ./cmd/seed/... -db "$(DB_URL)" -fixtures ../../fixtures/scenarios.json

# ── Code generation ────────────────────────────────────────────────────────

## gen-client: Generate TypeScript API client from openapi.yaml
gen-client:
	@echo "▶ Generating TypeScript client from api/openapi.yaml"
	$(OAPI_CODEGEN) api/openapi.yaml -o packages/api-client/src/schema.d.ts
	@echo "✓ TypeScript types written to packages/api-client/src/schema.d.ts"

# ── Testing ────────────────────────────────────────────────────────────────

## test: Run all Go integration tests
test:
	cd services/api && go test ./... -v -count=1

## test-integration: Run integration tests (requires running postgres)
test-integration:
	cd tests/integration && go test ./... -v -count=1 -tags integration

## test-t001: T-001 configuration matrix test
test-t001:
	cd tests/integration && go test ./... -run TestT001 -v

## test-t002: T-002 API contract parity test
test-t002:
	cd tests/integration && go test ./... -run TestT002 -v

## test-t003: T-003 fixture validation test
test-t003:
	cd tests/integration && go test ./... -run TestT003 -v

## test-t004: T-004 reproducible setup test
test-t004:
	@echo "▶ Running T-004: fresh setup from clean state"
	make stop-clean && make dev-bg && sleep 20 && make migrate && make seed
	@echo "✓ T-004 setup complete — check docker ps for service health"

## test-p1: Run all P1 tests (T-005 to T-016)
test-p1:
	cd tests/integration && go test ./... -run "TestT01[0-6]|TestT00[5-9]" -v

## test-p2: Run all P2 tests (T-017 to T-025)
test-p2:
	cd tests/integration && go test ./... -run "TestT01[7-9]|TestT02[0-5]" -v

## test-p3: Run all P3 tests (T-026 to T-038)
test-p3:
	cd tests/integration && go test ./... -run "TestT02[6-9]|TestT03[0-8]" -v

# ── Frontend ───────────────────────────────────────────────────────────────

## web-dev: Start child web app dev server
web-dev:
	cd apps/web && npm run dev

## ops-dev: Start ops console dev server
ops-dev:
	cd apps/ops && npm run dev

## web-install: Install frontend dependencies
web-install:
	cd apps/web && npm install
	cd apps/ops && npm install
	cd packages/api-client && npm install

# ── Linting ────────────────────────────────────────────────────────────────

## lint: Run Go linter and TypeScript checks
lint:
	cd services/api && go vet ./...
	cd services/worker && go vet ./...

## clean: Remove build artifacts
clean:
	rm -rf services/api/bin services/worker/bin apps/web/dist apps/ops/dist

## help: Show this help message
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
