# Implementation status — 11 September 2026

> Historical snapshot. The current implementation, evidence and remaining blockers are in [the 12 September audit](13-system-audit-and-completion-plan.md). The Docker and delivery findings below describe the earlier version.

This update follows the repository's `16c8665` prototype. The PRD describes intended behavior; existing code and checked boxes are not evidence that a phase has passed.

## Changes in this update

- Staff authentication now supports Keycloak through official `keycloak-js` and `coreos/go-oidc`, with an explicit issuer/subject-to-staff database mapping in migration 011. Local password login is demo-only.
- Staff and child JWT middleware reject the wrong token kind, missing identity/organization/role claims, missing expiry and alternative HMAC algorithms. Sensitive auth responses use `Cache-Control: no-store`.
- Organization access tokens are verified by a maintained OIDC library. Provider role claims do not grant application permissions; active database memberships are checked on each request. Ops credentials are kept in memory and refreshed by the SDK.
- Live Gemini assessment is no longer selected by configuration or possession of a key. Demo output is labeled; other modes preserve the human-help fallback. Assessment requests have a body limit and provider failures are not logged with submitted text.
- Container build contexts, Go builder versions, API migration inclusion, worker build path and container database addressing were corrected. Redis is optional; source mounts no longer mask the API binary.
- Product requirements, delivery/testing specifications and authentication reuse guidance are included. The complete documentation package requested earlier is still in progress; remaining documents are listed in the documentation index.

## Verification performed

| Check | Result |
| --- | --- |
| API `go test ./...` | Passed; database-dependent integration tests skipped because no database was configured. This does not establish P1–P3 gate completion. |
| Ops `npm run build` | Passed TypeScript checks and Vite production build. |
| Compose configuration validation | Passed. |
| Containers, migrations and worker end-to-end | Not run: Docker engine unavailable. |
| Real Keycloak login/MFA, browser sessions | Not run against a configured identity provider. |
| Real web push | Not demonstrated. |

## Next work in priority order

1. Run migrations and the integration suite against a disposable database. Change tests that skip on unexpected application errors into failures; only unavailable optional infrastructure may justify a clearly reported skip. Verify demo seed credentials and remove demo seed data from any real-use migration process.
2. Complete case/object authorization and alert publication checks, including same-organization scope, appropriate role, exact revision approval, transaction locks, idempotent activation and atomic withdrawal/cancellation. Current alert activation/withdrawal handlers need these repairs before a pilot.
3. Complete worker delivery wiring. Inspect transaction ownership and recheck consent, mode, revision and expiry immediately before sending. Record provider acceptance separately from observed device delivery. Keep `NOTIFICATION_MODE=DISABLED` until controlled testing is ready.
4. Strengthen return secrets, enforce rate limits, test shared-device/back-cache behavior, and verify independent routing, response ownership and safe-contact preferences.
5. Finish architecture/data, UX, safety/privacy, infrastructure/budget, operations, source register and AI handoff documents, mapping requirements to actual implementation evidence.
6. Configure and rehearse Keycloak; verify role revocation, staff offboarding, deployment origins and secret management. Establish staffed support, safeguarding procedures, lawful processing/retention and public-alert operating authority before any real-data beta.

These are engineering and operating release gates. The current push is a reviewed development update, not a production-readiness claim.
