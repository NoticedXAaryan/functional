# Data and API
## Storage responsibilities
Migrations 001–012 define organizations, staff and role bindings, entry points, private sessions, cases and case events, assignments/messages, alerts/revisions/approvals, subscriptions, outbox/jobs/attempts, tips and audit metadata.

Migration 012 adds registered alert areas, revision verification references, server-owned test classification, hashed subscription management capabilities and ambiguous delivery tracking. Existing subscriptions without a registered area and capability are intentionally not eligible for delivery.

Before applying to an existing deployment, back up and check for duplicate pending outbox rows or active subscription endpoints that could violate new unique indexes. Do not edit applied migrations to hide bad data.

## Important contracts
| Endpoint family | Responsibility |
| --- | --- |
| /sessions, /cases | Private session and durable report |
| /staff/cases and assignments/messages | Authorized human case workflow |
| /staff/alert-drafts | Organization-scoped draft creation/list/review |
| Alert activate / withdraw / resolve | Explicit lifecycle changes, authorization and transactions |
| /subscriptions/config | Mode, eligible registered areas, public VAPID key and enrollment availability |
| /subscriptions | Consent, browser keys, registered area, random management capability and test invitation |
| /public/alerts | Area-filtered current public projection |
| /public/alerts/{id} | Current status; terminal descriptions removed |
| Private tip endpoint | Minimized tip and opaque receipt |

See the [OpenAPI contract](../api/openapi.yaml) for exact routes. Historical portions still need full handler/contract reconciliation; a schema entry is not proof of an implemented behavior.

## Lifecycle invariants
Draft → independent approval → explicit activation → withdrawn/resolved/expired. Publication requires an unexpired approved revision, an active area, matching environment and a different approver. Terminal alerts cannot be reactivated through the existing activation endpoint.

Subscription mode is assigned by the server. Revocation requires the random management capability, not just a row ID. Area membership is an explicit subscription choice, not continuous GPS tracking.

## Required remaining data work
Persist/enforce safe contact and independent routing; replace return-secret scans with indexed high-entropy lookup; separate active credentials from return capabilities; make case/tip retries atomic and body-bound; implement retention/legal holds; enforce immutable approval binding and least-privilege runtime roles. Add migrations and behavioral acceptance for each, with rollback/data-preservation notes.
