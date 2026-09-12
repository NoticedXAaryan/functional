# Infrastructure and budget
## Hackathon
Use existing laptops, Docker Postgres, Go API/worker and local React servers. Incremental service spend can be zero if devices/internet already exist. No paid service purchase is required or authorized.

| Component | Local requirement | Pilot consideration |
| --- | --- | --- |
| Database | Docker Postgres/PostGIS | Backups, restore, access controls and capacity |
| API + worker | Local processes or Compose | Always-on hosting, monitoring, secret rotation |
| Web apps | Vite development servers | HTTPS static hosting and route fallback |
| Web Push | VAPID keys, browser consent | Supported devices, HTTPS, operational contact |
| Authentication | Seeded demo accounts | Keycloak hosting/OIDC provisioning and MFA |
| SMS/WhatsApp | Not implemented or needed for first demo | Separate paid provider and approved contact policy |
| Redis | Optional | Distributed rate limiting if selected |
| AI | Simulated guidance | Eligible provider and data review, not just an API key |

Current transport is direct Web Push through webpush-go. A browser endpoint hosted by FCM does not imply a Firebase project must be provisioned. Push-service acceptance does not guarantee timely delivery or OS display.

## Device constraints
Localhost supports development on the same computer. A phone needs a reachable HTTPS origin; its localhost is not your laptop. iOS/iPadOS Web Push requires a supported Home Screen web app. Denied permissions, battery/network conditions and OS policy can prevent delivery. Keep the current public alert page usable without push.

## Limits
TEST enrollment is invitation-based, default maximum 20 active recipients and cumulative 60 delivery jobs. Changing limits is an operating decision. Confirm what each configuration value actually enforces; a template variable alone is not an implemented control.

Before selecting hosting, collect region, uptime, expected cases/day, recipients/alert, retention and backup requirements. Compare live vendor pricing then; do not invent a fixed monthly total without those inputs. Budget separately for responders and safeguarding operations.
