# Bal Suraksha — Execution Plan & Task Tracker

> Historical implementation checklist. Checked boxes below indicate code was added, not that release gates passed. Current verification, authentication changes and remaining blockers are maintained in [docs/12-implementation-status.md](docs/12-implementation-status.md). P0–P3 are not yet verified complete. Live Gemini assessment is disabled.

## Phase 0 — Setup & Contracts ✅
- [x] Schema & DB Migrations 001–009
- [x] Configuration Mode Guards (`config.go`)
- [x] OpenAPI Contract (`api/openapi.yaml`)

## Phase 1 — QR Intake + Human Response ✅
- [x] Messages module (`messages/handler.go`)
- [x] Assignments module (`assignments/handler.go`)
- [x] Web UI child intake flow (`App.tsx`, `App.css`)
- [x] Integration tests T-005 through T-016 (`tests/integration/p1_gate_test.go`)
- [x] Conflict routing module (`routing/handler.go`)

## Phase 2 — Bounded AI + Incognito ✅
- [x] AI Assessment module (`assessments/handler.go`) with Gemini + Stub adapter
- [x] Interactive AI help assistant ("Is this okay?") tool in Web UI
- [x] Browser privacy / Cache-Control no-store headers (`index.html`)
- [x] Automatic 15-minute inactivity session teardown
- [x] Integration tests T-017 through T-025 (`tests/integration/p2_gate_test.go`)

## Phase 3 — Verified Alert + TEST Push ✅
- [x] Alerts approval & activation workflow (`alerts/handler.go`)
- [x] Area Subscriptions module (`subscriptions/handler.go`) with explicit consent
- [x] Background Worker Outbox Pipeline (`services/worker/main.go`)
- [x] Public alert status projection endpoint (`GET /api/v1/public/alerts/{id}`)
- [x] Integration tests T-026 through T-038 (`tests/integration/p3_gate_test.go`)
