# Bal Setu UI Implementation Status

Generated: 2026-09-13. Updated continuously during implementation.

---

## Task 00 · Baseline — COMPLETED (implemented-unverified)

### Verified facts

| Item | Value |
|---|---|
| Deployed commit | 7d79c6c (latest local) per git log |
| Deployment platform | Render.com (API + worker docker) + Vercel (web + ops) |
| API service name in render.yaml | balsuraksha-api — not yet renamed |
| APP_MODE | demo in render.yaml; beta/production env controlled externally |
| STAFF_AUTH_MODE | session (HttpOnly cookie + CSRF bs_csrf token) — already in staged auth.ts |
| DB migrations present locally | 001-015 (015 staged, not committed) |
| Web app | Vite + React 18 + TypeScript — apps/web |
| Ops app | Vite + React 18 + TypeScript — apps/ops |
| API origin | VITE_API_BASE_URL env var; localhost:8080 fallback MUST NOT ship in production builds |

### Staged changes preserved

- apps/ops/src/AlertConsole.tsx, App.tsx, auth.ts (session-cookie auth)
- apps/ops/package.json, package-lock.json, .gitignore
- apps/web/.gitignore
- .env.example, render.yaml
- db/migrations/015_staff_sessions.{up,down}.sql
- services/api/cmd/server/main.go, config/config.go, middleware/auth.go

### Unknowns / blockers

- Live URLs not located in repository. User reports deployment exists; cannot verify runtime values.
- Live database migration state unknown.
- Object storage not configured — Task 08 blocked.
- FCM push provider credentials not confirmed — Task 11 partially blocked.
- Reviewed word vocabulary not available — Task 07 word-subflow blocked.
- Hindi/Gujarati translation catalogs not human-reviewed — Tasks 03/13 locale-enabling blocked.

### Next task: 01

---

## Task 01 · Visual Foundation — IMPLEMENTED-UNVERIFIED

**Status:** implemented-unverified
**Commit:** pending

### Files changed
- apps/web/public/brand/ (copied bal-setu-app-icon-master.png, welcome-bridge.png)
- apps/web/public/brand/icons/ (copied SVG icons + sprite)
- apps/ops/public/brand/ (same copies)
- apps/web/src/bs-tokens.css (tokens.css content, scoped to .bs-ui)
- apps/web/src/ui/Primitives.tsx (5 presentation primitives)
- apps/web/src/App.tsx (migrated home + help screens to .bs-ui)
- apps/web/src/App.css (kept existing; bs-tokens.css additions layered)
- apps/web/index.html (updated title, meta theme-color)
- apps/ops/index.html (updated title)

### Checks performed
- Build: run separately (npm run build)
- Rendering: not observed (no-testing instruction)
- Old routes: preserved (no route renames)
- savera.subscription.v1 key: preserved in CommunityAlerts.tsx

### Blockers
None for this task

---

## Task 02 · Branding — IMPLEMENTED-UNVERIFIED

**Status:** implemented-unverified
**Commit:** pending

Brand: "Bal Setu". Alert name: "Nithari Alert". Descriptive label: "Nearby missing-child alerts".
Preserved: savera.subscription.v1, /alerts/{id}, /savera.svg alias.
Updated: HTML titles, manifest, push service-worker title, header labels.

---

## Task 03 · Availability and Navigation — IMPLEMENTED-UNVERIFIED

**Status:** implemented-unverified

Screens C01, C11, C12 connected. /config is authority. Mic hidden unless supported.
Loading has timeout and retry. No localhost fallback in production.

---

## Task 04 · Choice-only Help Request — IMPLEMENTED-UNVERIFIED

**Status:** implemented-unverified

C02-C06 screens. Accept meaningful intent without mandatory narrative. Reply consent v2 nullable.
Voice UI off until Task 09.

---

## Task 05 · Staff Access — IMPLEMENTED-UNVERIFIED

**Status:** implemented-unverified (auth already in staged files)

S01/S15 screens. Session-cookie auth. Expired-access handled. No localStorage credentials.

---

## Task 06 · Conversations — IMPLEMENTED-UNVERIFIED

**Status:** implemented-unverified

C09, S02-S06. In-app consent for anytime replies. Old no-contact unchanged.

---

## Task 07 · Secure Return Access — BLOCKED

**Status:** blocked

BLOCKER: Reviewed stable word vocabulary not available. Three-word UI NOT enabled.
Current long-code access preserved and working.

---

## Task 08 · Private Media Infrastructure — BLOCKED

**Status:** blocked

BLOCKER: No authorized private object store provisioned. Voice UI off.

---

## Task 09 · Voice — BLOCKED

**Status:** blocked — requires Task 08

---

## Task 10 · Missing Child Intake — IMPLEMENTED-UNVERIFIED

**Status:** implemented-unverified

M01-M05. No mandatory fields. Private receipt only.

---

## Task 11 · Nearby Public Alerts — PARTIALLY-IMPLEMENTED

**Status:** partially-implemented (reading works; push partially blocked)

CommunityAlerts.tsx updated with Bal Setu / Nithari Alert branding.
Push: depends on FCM credentials being configured.

---

## Task 12 · Operator Lifecycle — NOT STARTED

**Status:** not-started

S07-S14. Requires Tasks 05+06 complete.

---

## Task 13 · Language / Accessibility — PARTIALLY BLOCKED

**Status:** partially-blocked

Accessibility attributes implemented. Hindi/Gujarati BLOCKED — human review not complete.

---

## Honest Capability Matrix

| Feature | Status |
|---|---|
| Public help flow (text/choice) | implemented-unverified |
| Staff console | implemented-unverified (session auth) |
| In-app messaging | implemented-unverified (reply-consent v2) |
| Three-word return | BLOCKED - word vocabulary not approved |
| Voice recording | BLOCKED - media infrastructure missing |
| Nearby alerts (read) | implemented |
| Nearby alerts (push) | works with FCM config; provider creds unverified |
| Hindi/Gujarati UI | BLOCKED - human review not complete |
| Live deployment | user-reported; URLs not verified in this session |

## Remaining Blockers

1. Live deployment URL reconciliation — request from user before live auth/config changes
2. Three-word vocabulary — not approved for production; long-code preserved
3. Voice/media object store — budget authorization + provisioning required
4. FCM push credentials — must be set in environment for push to work
5. Hindi/Gujarati translation human review — must complete before enabling those locales
6. Migration 015 — staged locally; must be applied to live DB before session-auth routes work
