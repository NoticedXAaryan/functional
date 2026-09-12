# System audit and completion plan
Updated 12 September 2026.

> Subsequent beta engineering work is documented in [14 — Beta handoff](14-beta-handoff.md). Return access, contact preferences, assignment UI, tip review and submission idempotency below describe the earlier audit; read 14 for changes and the explicit no-testing verification limits.

## Assessment
The repository now contains a working synthetic alert vertical slice. It is not a complete child-safety service. A polished screen, an HTTP success response and a push-provider acceptance each prove different things; none proves that a child received human help.

The product has three audiences: a person privately asking for help; authorized staff handling reports; and community members voluntarily receiving local alerts. Keep those journeys distinct and link them through explicit actions.

## What exists
- React public application: account-free report creation, review, receipt, return-code entry, responder message display, simulated assessment and quick exit.
- React staff application: demo or Keycloak organization sign-in, organization case queue, case assignments/messages and alert preparation/review.
- Go API with Postgres/PostGIS: sessions, reports, staff permissions, case events, alert revisions and approvals, subscriptions, private tips and audit events.
- Go worker: transactional outbox expansion, area-matched delivery jobs, encrypted Web Push, bounded retries, expiry and escalation primitives.
- Savera Alert public experience: generic dawn emblem, area selection, subscription consent and revocation, current alert status, private tip receipt and responsive layout.
- Migrations 001–012, local Docker infrastructure, API/worker tests, browser rehearsal script and CI workflow.

Savera is a working product name. It is not presented as an official Indian emergency network or named after a victim whose family has not been consulted.

## Integration repairs already made
| Failure | Repair |
| --- | --- |
| Worker did not dispatch notifications | Dispatcher wired into background loop |
| Push attempted plaintext payloads | Established webpush-go library encrypts using browser keys and signs VAPID requests |
| Geographic matching fields were never populated | Explicit registered-area IDs now connect drafts, subscriptions and matching |
| Alert state changes and outbox could diverge | Transactional approval/activation/withdrawal, deduplication and dispatch-time checks |
| Weak organization and role checks | Draft ownership, separate approver, activation and terminal-action authorization |
| Revocation by subscription ID alone | Random management capability required; hash stored server-side |
| Arbitrary push endpoint requests | HTTPS provider allowlist, no redirects, bounded timeout |
| Terminal public alerts retained details | Public description and tip entry removed on closure/expiry |
| Credentialed CORS allowed arbitrary origins | Exact configured origins |
| Migration paths failed on Windows | Filesystem migration source |
| Responder messages fetched but invisible | Messages rendered in public return view |
| Technical navigation hid choices | Plain-language home navigation, local-alert link and in-context staff guidance |

## Usability changes in this update
The home screen explains the next action and report lifecycle. “Ask for help” leads directly to intake; assessment is optional and explicitly simulated. “Check a report” and Savera Alert are discoverable from home. Staff sign-in explains the demo roles. The queue explains how cases arrive and what assignment means. Keyboard users can open case rows. Focus indicators and wrapping navigation improve small-screen use.

Further experience work: accessible native selection controls throughout intake; associated labels for every field; translated screens reviewed by native speakers; status language without internal codes; unsent-change warnings; assignment creation and safe-contact visibility. These are tracked work, not completed claims.

## Remaining launch blockers
| Priority / owner | Required work | Acceptance before real data |
| --- | --- | --- |
| P0 Backend/security | Replace four-word return lookup. Current implementation scans session hashes and failed attempts affect multiple sessions | High-entropy indexed capability; per-capability abuse controls; one person's wrong guesses cannot lock out others |
| P0 Backend/product | Separate quick-exit session credential revocation from retained return access | Exit invalidates current credential without accidentally destroying intended return access; back navigation cannot restore private content |
| P0 Safeguarding/backend | Persist and enforce safe-contact preferences; independent routing for institutional conflicts | Every report has an accountable appropriate destination; unsafe contact is prevented |
| P0 Backend | Case/tip submission concurrency and idempotency | Simultaneous retries create one result; conflicting request bodies cannot reuse a key; return path cannot be overwritten |
| P0 Auth/security | Finish least-privilege case grants, role consistency and real OIDC/MFA setup | Cross-organization and unassigned access denied; actual provider sign-in and role removal rehearsed |
| P0 Operations | Named on-duty responders, acceptance/escalation ownership and realistic response promises | Missed acceptance reaches a staffed fallback and is acknowledged |
| P0 Security/operations | Retention, deletion/legal holds, log redaction, backup restore, incident response and independent review | Documented policies implemented and rehearsed |
| P0 Alert operations | Real issuer authority and area/jurisdiction administration | Approved operators and publication protocol exist; no automatic public release from a private report |
| P1 Notifications | Observe actual device delivery, closed-browser behavior and invalid subscriptions | Record provider acceptance separately from device observation across supported devices |
| P1 Platform | HTTPS deployment, secrets management, monitoring and distributed abuse controls | No secrets in client assets; capacity limits and rollback rehearsed |
| P1 Backend | Alert revision correction flow, immutable approval binding and migration preflight | Edited material requires new review; old queued revisions never deliver |
| P1 Product | Verified QR placements, full assignment UI, staff tip-review UI and reviewed translations | Revoked placement fails safely; staff can handle cases and incoming tips without direct database edits |

Do not enable real child-facing inference until provider eligibility, data handling and safeguarding review are complete. The current assessment is a demo stub.

## Rollout sequence
1. **Connected synthetic demonstration:** complete navigation, clear copy, reproducible setup and fictional report/alert walkthrough. No real reports.
2. **Core case repair:** return capability, session exit, safe-contact storage/enforcement, assignment and conflict routing, atomic submissions. This is the next engineering milestone.
3. **Invited notification rehearsal:** at most 20 consenting adult testers, TEST labels, configured keys and a staffed test operator. Observe a device; exercise withdrawal/revocation and outages.
4. **Closed pilot readiness:** organization OIDC/MFA, jurisdiction, operating agreements, privacy/retention, incident response, backup restore, accessibility and independent security review.
5. **Limited staffed pilot:** bounded area/hours and metrics; expand only after actual response evidence. Flutter and broad distribution are later.

Each phase needs an owner, explicit acceptance evidence and a rollback decision. The PRD and delivery plan contain the larger requirement backlog; the blockers above take precedence over visual polish for a real-data release.

## Evidence and limits
Earlier in this implementation session: all twelve migrations applied to a fresh local Postgres database; API and worker suites passed; both frontend builds passed; a browser rehearsal exercised preparation by one account, approval/publication by another, a private tip and withdrawal; desktop/mobile screenshots were captured under artifacts/evidence.

Those observations apply to the version exercised then. After the latest navigation/documentation changes were connected, both frontends passed their TypeScript checks and production builds, and the diff whitespace check passed. No new test suite was added for these interface changes. The earlier browser rehearsal was not repeated for the latest copy/navigation. No real push-device receipt, production OIDC sign-in, live deployment or staffed intervention has been verified.

## What “complete” means
A report is complete operationally only when it reaches an appropriate accountable person, preserves the reporter's constraints and supports a safe response. An alert is complete only when its authority, audience, expiry, withdrawal and tips workflow all work. The remaining work is concentrated at those boundaries.
