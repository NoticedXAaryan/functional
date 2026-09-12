# Bal Suraksha / Savera Alert

A child-safety prototype with three connected spaces: **private help**, **a staff workspace**, and **local community alerts**.

The interface uses a light colour palette, four clear home choices, and layouts that adapt to phone screens. Each workspace links to the others. Notification setup stays hidden when the service is switched off. For hosted deployments, set `VITE_OPS_URL` in the public app and `VITE_PUBLIC_URL` in the staff app to the corresponding frontend addresses; local defaults are ports 5174 and 5173.

**Current status:** a functioning synthetic demonstration, with important gaps before real-world use. The public app is not an emergency dispatch service. Use fictional reports and invited adult testers. [Read the audit and completion plan](docs/13-system-audit-and-completion-plan.md).

## Which screen do I open?

| I want to… | Open | What happens |
| --- | --- | --- |
| Ask for help or report a missing child | [Public app](http://localhost:5173) | Submit a private report without an account |
| Explore “Is this okay?” | Public app → Is this okay? | See simulated guidance; this is not a live safety assessment |
| Check a submitted report | Public app → Check a report | Enter the private return words; this remains a demo feature |
| Review reports or prepare an alert | [Staff workspace](http://localhost:5174) | Sign in, open Cases or Alert Review |
| Read local alerts or manage notifications | [Savera Alert](http://localhost:5173/alerts) | Choose a registered area, read current alerts and optionally enroll this browser |
| Share a sighting | Open an active alert → private tip form | Save a tip and receive a receipt |

A missing-child **report is private**. It does not automatically publish an alert. An authorized preparer creates an alert, a different approver reviews it, and an explicit publish action activates it.

## Run the application on your computer

You need Docker Desktop running, Node.js 22 with npm, and an internet connection for the initial dependency downloads. Go 1.26 is needed only for local backend development and key generation.

Run all commands from the repository folder unless a step says otherwise. Use separate terminals for the two frontends.

### 1. Start the database, API and background worker

```powershell
docker compose up -d --build
```

This starts Postgres on port **5454**, the API on **8080**, and the worker. The API applies database migrations and seeds fictional demo accounts. Docker Compose defaults to demo mode with notifications **disabled**. Redis is optional and is not required for this walkthrough.

If you already have a root `.env`, its settings can override these defaults. Keep `APP_MODE=demo`, `STAFF_AUTH_MODE=demo`, `NOTIFICATION_MODE=DISABLED`, and `AI_ENABLED=false` for your first walkthrough.

### 2. Start the public app

In a new terminal:

```powershell
cd apps/web
npm install
npm run dev -- --host localhost --port 5173 --strictPort
```

Open **http://localhost:5173**. Leave the terminal running.

### 3. Start the staff workspace

In another terminal:

```powershell
cd apps/ops
npm install
npm run dev -- --host localhost --port 5174 --strictPort
```

Open **http://localhost:5174**. Both frontends connect to `http://localhost:8080/api/v1` by default.

For a different API address, set `VITE_API_BASE_URL` in each application's `.env.local` and restart its development server. Update API `WEB_ORIGINS` for the exact frontend origins. A phone's “localhost” is the phone itself, not your computer.

## Your first five-minute walkthrough

1. Open the public app and choose **Ask for help**.
2. Choose **Report a missing child** and enter an explicitly fictional example. Review and submit it.
3. Keep the receipt and return words privately. A receipt means the database saved the report, not that a responder has read it.
4. Open the staff workspace. Sign in as **preparer.demo**, password **demo-password**.
5. Open **Cases** to see the report. Open **Alert Review** to create a draft: choose the missing-child case and fictional area, add an appropriate public description, issuer, verification reference and expiry.
6. Sign out. Sign in as **approver.demo**, password **demo-password**, and approve the draft. The preparer cannot approve their own draft.
7. Publishing is intentionally unavailable when notifications are disabled. Follow the [invited notification rehearsal](docs/09-operations-runbook.md) to enable TEST mode first.
8. After publication, open **Savera Alert**, choose the matching area, open the alert and submit a fictional private tip.
9. Withdraw or resolve the alert in the staff workspace. Refresh its public page: the description and tip form are removed.

Demo credentials are seeded for local fictional use. They are not production accounts. Organization sign-in uses Keycloak OIDC when configured; see [authentication setup](docs/11-reuse-and-authentication.md).

## What is useful today, and what is unfinished?

| Feature | Current value | Important limit |
| --- | --- | --- |
| Private report + receipt | Real database-backed intake | Routing, safe-contact enforcement and return security need completion |
| Staff cases and messages | Separate operating workspace | Assignment workflow and staffed escalation remain incomplete |
| Savera Alert | Separate approval, registered areas, public status, private tips | No government authorization or nationwide broadcast integration |
| Web Push | Encrypted delivery implementation, durable queue, retries, revocation | Real-device receipt still needs an observed rehearsal |
| “Is this okay?” | Demonstrates a voluntary guidance flow | Simulated output, not live AI |
| Quick exit | Leaves the screen quickly | Does not erase history, device copies or retained reports |
| QR entry | Demonstration entry experience | Verified physical placement and routing are not connected |
| Languages | Captures language preference | Most screens remain English |

The strongest current feature is the **private report → staff review → approved local alert → private tip → withdrawal** flow. The biggest remaining work is reliable human case handling and protection of return access.

## Notifications: do I need to pay?

The current implementation uses standard browser Web Push with VAPID keys and the open-source webpush-go library. It does not require a paid SMS plan or a Firebase project. Hosting, a domain and operational support may cost money; browser delivery is not guaranteed. Phones need HTTPS and supported browser permissions. iPhone/iPad users need a supported Home Screen web app installation. [Infrastructure and budget](docs/06-infrastructure-and-budget.md) explains the limits.

## Stop, restart and troubleshoot

- Stop backend containers: `docker compose stop`. Restart: `docker compose up -d`.
- Stop a frontend: press Ctrl+C in its terminal.
- See backend startup errors: `docker compose logs --tail 80 api worker`. Avoid sharing logs containing private data.
- “Failed to fetch”: confirm the API is running, frontend API URL matches, and exact origin is allowed.
- Port already used: stop the older process or consistently change both frontend URL and API CORS settings. Do not run Docker and a local API on port 8080 together.
- Empty queue: submit a report first, use the correct organization account, and Refresh.
- Cannot publish: check TEST mode, separate approver, approved/unexpired draft and active registered area.
- Cannot enable notifications: check invitation, consent, keys, browser permission, HTTPS and OS support. Reading alerts does not require notifications.
- Do not run `docker compose down -v` unless you intentionally want to erase the local database.

## Project map

`apps/web` public React app · `apps/ops` staff React app · `services/api` Go HTTP API · `services/worker` delivery/background work · `db/migrations` schema · `api/openapi.yaml` API contract · `docs` requirements and handoff.

Start with the [documentation index](docs/README.md), [current report](docs/13-system-audit-and-completion-plan.md), and [AI handoff](docs/ai/START-HERE.md). Historical plans describe intended behavior; they are not evidence that every feature is implemented.
