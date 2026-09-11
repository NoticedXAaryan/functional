# Reuse and staff authentication

Updated 11 September 2026. The repository now includes a Keycloak integration using its official JavaScript SDK and the maintained Go OIDC verifier. Provider deployment and a real sign-in rehearsal are still required; this is not a completed production rollout.

## Decisions

| Concern | Reuse | Product-specific responsibility |
| --- | --- | --- |
| Staff sign-in, MFA, password recovery | Keycloak; official `keycloak-js`; `coreos/go-oidc` | Explicit staff, organization and role grants in PostgreSQL |
| HTTP routing | Existing chi middleware | Object-level authorization on every case, alert and tip |
| Database access/migrations | Existing pgx and golang-migrate | Transactions, approved state transitions, retention and audit rules |
| Push | A maintained Web Push library or official FCM SDK when delivery is completed | Consent, area selection, expiry, cancellation, retry limits and accurate delivery states |
| UI | Existing React/TypeScript and native browser controls | Accessible wording, safe contact choices, failure and recovery states |

Prefer an established implementation when it removes protocol or security maintenance. Evaluate maintenance history, license, security advisories, data handling and operating cost; pin dependencies and retain lockfiles. Do not add a second authentication framework or build password recovery, MFA, OAuth redirects, token refresh or push encryption from scratch. Keycloak has no software license charge, but hosting, patching and staff time still cost resources.

## Configure Keycloak

1. Use a dedicated realm. For a local synthetic demo, follow the official Docker quickstart. `start-dev` is a development mode. A beta needs HTTPS, persistent storage, backups and an operator responsible for updates.
2. Create a public OIDC client named `balsuraksha-ops`. Enable Standard Flow with PKCE S256; disable client authentication, implicit flow and direct access grants. Register exact redirect URLs and web origins, for example the actual local ops URL `http://localhost:5174/`. Avoid wildcard redirects. Require staff MFA for the pilot in Keycloak.
3. Configure an audience mapper so access tokens issued to that client include `balsuraksha-api` in `aud`. The API requires the issuer, audience, signature, expiry, nonempty subject, Keycloak access-token claim `typ=Bearer`, and authorized party `azp=balsuraksha-ops`.
4. Set API configuration: `STAFF_AUTH_MODE=oidc`, `OIDC_ISSUER=https://YOUR-IDENTITY-HOST/realms/YOUR-REALM`, `OIDC_AUDIENCE=balsuraksha-api`, `OIDC_CLIENT_ID=balsuraksha-ops`. HTTPS is required outside demo mode. HTTP localhost with an explicit port is accepted only for a local demo. Discovery must be reachable from the API and the issuer must also be reachable by the browser. Do not replace the issuer with a Docker-only hostname.
5. Apply migration 011. An administrator provisions a `staff_members` record, an explicit `role_grants` record for the same organization, and a `staff_identities` record binding that staff ID to the exact issuer and immutable Keycloak user subject. The provider-only staff password may be NULL. Never map by an email address alone or grant access automatically on first login. Keep actual identity IDs and administrative SQL outside the public repository.
6. Set `VITE_API_BASE_URL` in `apps/ops/.env.local` if the API is not `http://localhost:8080/api/v1`. Rebuild the frontend after changing it. The API currently allows the local web origins in its CORS configuration; configure exact deployed origins before a hosted pilot.
7. Rehearse sign-in, refresh, logout, invalid audience, revoked identity and revoked role with synthetic cases. Revoke `staff_identities.revoked_at`, `role_grants.revoked_at`, or disable the staff record to remove app access. The API rereads active grants on every OIDC request. Provider logout alone does not necessarily revoke an already-issued access token immediately; use short token lifetimes and application revocation for urgent removal.

The current implementation selects one deterministic role for a staff member: admin, supervisor, alert approver, alert preparer, then responder. Use separate test identities for separate duties. A future multi-role design must specify permission aggregation explicitly and preserve separation of preparation and approval.

## Current behavior and verification limits

`GET /api/v1/staff/auth/config` advertises sign-in mode and public provider configuration. The ops app delegates organization sign-in to Keycloak and keeps tokens in memory. `GET /api/v1/staff/auth/me` returns the application's active staff mapping. SDK token refresh happens before protected requests. Unmapped users receive no staff access. Local password login is rejected in beta/production and whenever OIDC mode is selected; demo passwords remain a synthetic-only convenience.

Go tests exercise signed OIDC tokens and application identity lookup boundaries. The ops production build passes. A running Keycloak provider, database migration rehearsal, MFA/recovery and browser sign-in have not been verified in this environment. Existing case/alert authorization and delivery gaps still prevent a real-data pilot; replacing authentication does not resolve those workflows automatically.

## Primary references

- [Keycloak JavaScript adapter](https://www.keycloak.org/securing-apps/javascript-adapter)
- [Keycloak Docker quickstart](https://www.keycloak.org/getting-started/getting-started-docker)
- [Keycloak container operations](https://www.keycloak.org/server/containers)
- [Go OIDC verifier](https://github.com/coreos/go-oidc)

Gemini can assist an adult developer with code. It is not an eligible child-facing runtime in this implementation: the [Gemini API terms](https://ai.google.dev/gemini-api/terms) restrict API clients directed toward or likely accessed by under-18s. Demo assessment uses a labeled stub; beta/production expose the human-help fallback, and `AI_ENABLED=true` fails startup.
