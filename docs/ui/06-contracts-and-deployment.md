# 06 · Contracts and deployed integration

Status: proposed additive contracts, not implemented endpoints. Read the actual deployed schema before choosing migration numbers; local migration 015 already exists in staged work. Do not edit or rerun applied migrations to achieve these changes.

## Deployment evidence and current gaps

User reports the application is deployed. The separately requested strategy document/live URLs were not available during this writing pass. Inspected deployment-related sources: root README, docs/14, `compose.beta.yaml`, `render.yaml`, both frontend `vercel.json` files and current local auth/config code. Do not assume the historical Compose plan is the live topology.

| Observed local source | Implication for the implementation |
| --- | --- |
| Vercel configs currently only rewrite to SPA HTML | They do not configure an API proxy; a relative API URL would currently risk receiving HTML |
| Both frontends default API to localhost | Production build must fail if public API configuration is missing/localhost |
| `render.yaml` has APP_MODE=demo, static demo signing secret, disabled push and worker plan free | These are not acceptable beta defaults; actual live environment values are unknown. Render free workers are unsupported according to its free-service documentation |
| Current auth uses HttpOnly staff session cookies and a readable CSRF cookie | Separate Vercel and Render domains cannot share readable CSRF cookies; SameSite and browser restrictions can block the staff session |
| `HandleLogin` passes `r.RemoteAddr` into an INET column | RemoteAddr commonly includes a port; parse safely before inserting, handle proxies only from trusted infrastructure |
| `RequireStaffSession` reads stored role without rechecking active grants | Add current staff/org/membership checks so revoked roles do not retain access until expiry |
| Server expiry is extended with an absolute cap, cookie lifetime is separate | Align cookie refresh and server expiry; check absolute expiry before permitting requests |
| Return access is a 256-bit code | Add device/card support without retiring existing valid codes |
| Contact preference uses message/no_contact and safe hours | Split permission from timing; preserve restrictive legacy consent |
| Existing public intake requires text in the experience | Add intent-only and ready-audio alternatives through UI and API validation |
| Notifications currently select area IDs | Do not advertise automatic live GPS proximity |
| Media storage/processing absent from inspected flows | No voice button that records but cannot securely deliver |
| Caddy policy disables microphone and geolocation | If that path is deployed, explicitly permit self for intended voice/location routes; otherwise capabilities will fail |

These are code observations, not claims of an exercised live failure. Preserve all unrelated staged work. Do not solve cookie problems by putting staff secrets into browser storage, disabling CSRF, accepting wildcard origins or reverting to demo login.

## Recommended integration contract

Keep React/TypeScript public and staff apps, Go API/worker and PostgreSQL/PostGIS. Use the existing deployed providers if they support the requirements; do not replatform simply for design work. Private object storage is the new infrastructure requirement for voice.

For Vercel frontends and a remote Go API, use a **same-origin `/api/v1` proxy on each frontend hostname**. Order the API rewrite before the SPA fallback. Configure API cookies as host-only Secure cookies with appropriate SameSite and Path; do not hardcode a Render cookie domain. Preserve Set-Cookie, CSRF headers and no-store response headers through the proxy. Validate exact public/staff origins server-side and trust forwarded headers only from the deployed proxy. Keep child and staff session names separate.

Alternative: one reverse-proxy hostname serving `/`, `/staff/` and `/api/v1` as in Compose. Either topology must be explicitly selected from live strategy; do not mix origin assumptions. New custom domains require a plan for existing device credentials and subscriptions bound to the old origin. Keep old access available through expiry or provide authenticated migration; changing a URL cannot move a cookie or browser push subscription.

Production headers: no-store for private HTML/API/audio, no-referrer, nosniff, frame denial, a reviewed CSP including exact API/media origins and local blob playback where required, microphone/geolocation self only where used, no cross-origin embedding. Public hashed static assets may be cached; private responses and identifying expired alert data must not enter service-worker caches. Do not put secrets in VITE variables.

## Proposed API additions

Reuse canonical `/api/v1` routes; names below are relative. All private responses are no-store. Existing return/access contracts remain during migration. Complete OpenAPI before UI implementation.

| Method/path | Request essentials | Response / authorization |
| --- | --- | --- |
| GET `/config` | None | Existing fields plus `capabilities`, `operator_display_name`, reviewed languages, availability text and public feature state; no private configuration |
| POST `/cases` | Session bearer, Idempotency-Key; intent, optional text, ready attachment IDs, language, entry point, reply permission | 201 durable receipt. Accept intent-only. Route to actual team before receipt; replay exact request returns same result |
| POST `/cases/{id}/messages` | Existing child session; operation ID, optional text, ready attachment IDs | At least text or audio, same-case auth; commit attachment association/message atomically |
| POST `/staff/messages` | Staff session+CSRF; case ID, text/audio, note/reply type, summary | Existing scope and explicit child reply permission; notes cannot be attached as child-visible messages |
| PATCH `/cases/{id}/reply-preference` | Child session, expected version, `in_app_replies_allowed` | Store event/version. No implicit external contact consent |
| POST `/return-devices` | Active child session, word IDs/version and explicit persistence choice | Device credential in HttpOnly cookie; no body echo; return expiry and success state |
| POST `/session-access/device` | Cookie proof + words, Origin/CSRF | Short active access after scoped verification; generic denied response |
| DELETE `/return-devices/current` | Device/session proof + CSRF | Revokes persistent proof; idempotent; does not delete case |
| POST `/return-cards` | Authenticated child access, operation ID | Reveal random card secret once through secure response; digest stored; retry design must not generate orphan/unknown credentials |
| POST `/session-access/card` | Recovery capability by POST | Issue short active access, throttle and audit without logging secret |
| POST `/attachments` | Authenticated owner/session, media type, byte count and purpose | Upload ID, short-lived scoped upload capability, limits; case/session ownership fixed server-side |
| POST `/attachments/{id}/complete` | Checksum and operation ID | Accepted processing state; server validates object rather than trusting browser |
| GET `/attachments/{id}` | Same owner/case authorization | Processing metadata; no object-store secret |
| GET `/attachments/{id}/content` | Fresh authorized session | Private no-store streaming with Range support |
| DELETE `/attachments/{id}` | Owner of unsent attachment | Cancel/orphan cleanup; submitted evidence deletion uses separate retention procedure |
| POST `/locations/resolve-area` | Optional one-time coordinate or locality text | Candidate supported areas; do not log raw coordinates |
| PATCH `/subscriptions/{id}` | Management capability, area IDs, operation ID | Atomic confirmed subscription area/version; cannot alter another endpoint |
| POST `/staff/tips/{id}/review` | Scoped staff, expected version, follow-up status | Private audited review state |

Common errors: 400 invalid choice, 401 return/sign-in needed, 403 unavailable permission, 409 operation/version conflict, 413 clip too large, 415 unsupported recording, 429 cooldown with Retry-After, 503 service unavailable. Frontends map codes to reviewed local-language text and retain unsent content. Never display raw server errors with personal data. Generic failures must not reveal whether a return phrase or case exists.

All mutation retries use a cryptographically random operation ID scoped to actor+action, with a canonical request digest. Same key plus different body is 409. A lost response must not create duplicate reports, messages, attachments or alert jobs. Client Send is disabled only while that operation is pending; it must recover after timeouts.

## Data additions and migration constraints

| Entity/change | Minimum fields and constraints |
| --- | --- |
| `return_devices` | ID, private session/case FK, credential digest unique, word verifier/salt, wordlist version, expiry/revocation, consent timestamp; no raw words or secret |
| `return_cards` | ID, owner FK, unique digest, created/expiry/revocation, replacement reference; separate from public receipt |
| Reply preference v2 | Nullable in-app permission during migration, external contact policy separately, source/version/timestamps; old values preserved until explicit update |
| `attachments` | UUID, owner/session/case, private object key, checksum, detected MIME/bytes/duration, state, created/expiry, retention/hold marker |
| Message attachment association | Message FK + attachment FK unique per intended association; note visibility inherited, not independently guessable |
| Geographic areas | Versioned valid geometry, name translations, jurisdiction, active/reviewed status |
| Tip review events | Tip ID, authorized reviewer, time, status/version and minimal follow-up metadata |
| Availability/escalation | Configured team/coverage, worker heartbeat, acknowledgments and fallback target; no fake staffing derived from role count |

Do not store voice in Postgres. Use least-privilege service roles, encryption and backup appropriate to each store. Migration order is expand schema → deploy compatible readers/writers → optional controlled backfill → enable feature → later contract. Never drop old credential columns or assume seeded organizations are real during backfill.

## Hosting and spend

Record live public URL, staff URL, API URL, worker provider/service, DB region, object storage region, auth mode, deployed commit, migration version and secret owner in a private operator release record. Public docs may contain verified URLs but never credentials or database connection strings. An unclaimed temporary database is not a durable deployment: establish ownership and expiry/billing policy before child data.

Voice planning estimate: at 32 kbit/s, 120 seconds is approximately 480 kB before container overhead. 1,000 clips are roughly 480 MB; originals, derivatives, backup and playback bandwidth multiply this. Enforce 8 MiB per-clip hard cap regardless of target encoding. Set storage/upload quotas and staff-visible failure alerts without blocking text help when voice quota is exhausted. Disable transcription initially to avoid cost and private-data transfers.

Free-tier boundaries change. The current [Render free-service documentation](https://render.com/docs/free) excludes background workers, sleeps idle free web services and expires its free Postgres after 30 days. Neon is a separate database service; inspect its actual project limits and PostGIS availability before assuming the Render DB limits apply. Hosting on Vercel does not by itself run this continuous Go worker or store private audio. Any paid resource needs the user's budget authorization; do not hide ongoing costs behind “free beta.”

## Pending strategy reconciliation

Required before implementation deploy: obtain the user's referenced deployment strategy/live URLs; identify actual commit and configuration without exposing secrets; decide same-origin routing; establish persistent DB and worker ownership; record actual authentication mode; confirm operator and independent routing; then update this section with evidence. If the strategy is supplied later, preserve its valid topology and amend only contradictions with these required behaviours.
