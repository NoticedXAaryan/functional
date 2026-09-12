# Limited beta handoff
Updated 12 September 2026. This is the current handoff; it supersedes older implementation-status claims.

## Implemented in this update
- A 256-bit private return code replaces the old four-word lookup. Only its SHA-256 digest is stored and indexed. Failed guesses do not update other sessions. The return capability expires after 30 days; returning rotates the active JWT and restores its 15-minute lifetime.
- Quick exit requests revocation of the current credential, without deleting the case or its return capability. The browser request is best effort if connectivity fails; JWT expiry remains the fallback. A restored back-forward-cache page reloads instead of continuing the previous React state.
- Report submission locks the session, binds the idempotency key to the request body and prevents a second report from overwriting that session's return path.
- Contact preferences are persisted. Non-note staff replies are blocked for no-contact cases, closed cases, missing preferences and times outside the allowed window. Supported zones: Asia/Kolkata and UTC. Equal start/end means any time; overnight windows are supported. This controls in-app messages only; no external contact channel has been added.
- In beta, intake requires an explicit enabled flag, a random invitation, a configured active entry point and an organization with an active supervisory role. Conflict reports go directly to the entry point's configured independent organization; missing independent routing rejects the submission before saving.
- Private narrative and message access requires supervision, active assignment or an explicit case grant. Alert roles alone do not grant private case access. Responders' queues are limited to assigned/granted cases; alert roles see the organization queue metadata needed to select alerts.
- Supervisors can choose an active responder in the UI. Staff see the report narrative and contact preference. Acceptance and assignment update their case state transactionally.
- Reporters can add a message from the return-access screen; staff replies remain governed by the saved contact choice.
- The public navigation now offers a real concern-reporting path instead of the simulated guidance preview. Child-facing live AI remains unavailable.
- Authorized supervisors/alert reviewers can open private tips in Alert Review. Tip submissions use transactional, body-bound idempotency and check active approved/unexpired alerts.
- Public configuration drives intake availability and beta invitations. QR links use /?entry=OPAQUE_ID and resolve through the API.
- A separate HTTPS Compose deployment serves public UI at /, staff UI at /staff/, and API at /api/v1 on one domain. Database and API ports are not exposed by that deployment. Local development remains on the existing ports.

## Migration impact
Apply 013 and 014 through the API migration runner. They preserve existing reports and tips.
**Old four-word codes are no longer accepted.** They cannot be converted securely because the original plaintext was not stored. Existing cases remain accessible to authorized staff. New submissions receive the long code. Existing active session tokens must match their stored JTI.
Legacy cases without contact preferences default to no outreach. Do not silently assign a contact preference to real legacy reports.
The old duplicate /staff/cases/{id}/messages and /assignments write routes are retired; use /staff/messages and /staff/assignments.

## Deploy with your real organization
1. Obtain a server with Docker, a DNS hostname pointing to it, and inbound 80/443 available. No cloud account or paid resources were created by this update.
2. Copy .env.beta.example to the ignored .env.beta.local. Replace every placeholder. Use generated independent database/session secrets; URL-encode any special characters in DATABASE_URL. The database hostname inside Compose is postgres.
3. Configure an HTTPS Keycloak realm and the public browser client. Exact redirect URI: https://YOUR_HOST/staff/*. Web origin: https://YOUR_HOST. Set the API audience and map real issuer/subject identities to application staff records as described in docs/11-reuse-and-authentication.md. Require MFA through the identity provider.
4. Start with BETA_INTAKE_ENABLED=false and NOTIFICATION_MODE=DISABLED:
   docker compose --env-file .env.beta.local -f compose.beta.yaml up -d --build
5. Provision actual organizations, venue/entry-point records, staff memberships, role_grants and OIDC identity bindings. Do not reuse the fictional seed organization or demo identities. Set entry_points.independent_organization_id to an independent staffed organization, distinct from the local organization. Set BETA_ENTRY_POINT_ID to the actual entry point UUID. It is distinct from the opaque_id in QR URLs.
6. Set a random BETA_INVITATION_CODE of at least 24 characters and distribute it to invited participants through the responsible organization. This shared invitation limits intake; it is not individual identity or an authorization system for staff.
7. Name the on-duty responders, independent fallback, operating hours and response responsibilities. Complete the remaining operational items below before enabling real intake.
8. Set BETA_INTAKE_ENABLED=true and recreate the API only when the organization is ready. UI notices derive from APP_MODE, not a manual removal of fictional labels.
9. Keep broadcasts off until real issuer authority, registered non-test alert areas, consented subscriptions and valid VAPID keys/contact are configured. LIVE uses non-test data; it does not turn fictional alerts into real ones.

The deployment is intentionally separate from local Compose and uses separate volumes. Automatic Caddy certificates require a reachable domain. The stack does not configure DNS, host firewall, off-host backup, identity provider or a government integration for you.

## Still required before real child data
- An actual staffed operator, service hours, independent fallback and publication policy.
- Configured identity provider and per-person memberships, including removal/MFA procedures.
- Reviewed privacy notice, retention/legal-hold/deletion implementation, access audit handling, backup/restore and incident process.
- Distributed abuse protection and capacity planning. Body limits and random invitations are not a complete anti-abuse system.
- Operational escalation delivery/acknowledgement; current worker escalation records are not a staffed notification channel.
- Full case closure/referral UI, remaining permission review, translated content and accessibility review.
- Real device notification observation and deployment configuration review when permitted.

## Verification status and user constraint
No test suites, browser rehearsals or generated example reports were run for this beta update, as requested. Go API compilation and both frontend production builds completed. Local Docker images built and migrations ran as part of service startup. These are build/startup results, not end-to-end assurance.
The dedicated beta HTTPS stack has not been deployed to an actual domain. Real OIDC authentication, operator routing, a live push device and the revised workflows have not been behaviorally verified in this update.
Existing test fixtures/contracts may require updates for the new secure return credential, canonical endpoints and stricter permissions before running the old suite. CI is skipped for this commit to honor the no-testing request; its workflow remains available for a future verification phase.

## Important limits
This is a substantial beta foundation, not a claim of production readiness. Do not infer operational coverage from an enabled database role, or that a report was read from a saved receipt. External setup and the remaining operational/privacy work cannot be replaced by changing the UI label.
