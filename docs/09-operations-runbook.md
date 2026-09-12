# Operations runbook
## Invited fictional push rehearsal
Only consenting adult testers and fictional alerts. First complete the disabled-mode walkthrough in the root README.

1. With Go 1.26 installed, from services/worker run `go run ./cmd/dev-config -out ../../.env.demo.local`. It refuses to overwrite an existing file. Keep the generated private key, session secret and invitation out of git/logs.
2. Review that local file. Set a valid operator-controlled VAPID contact before external testing. Keep APP_MODE=demo, NOTIFICATION_MODE=TEST_ALLOWLIST and AI_ENABLED=false.
3. For Compose, copy its reviewed settings into the ignored root .env (preserve any existing settings deliberately; do not blindly overwrite). Compose reads .env for both interpolation and container configuration. Using only --env-file is not a substitute for the service env_file settings.
4. Restart API and worker with `docker compose up -d --build --force-recreate api worker`. Ensure an older local API is not occupying port 8080.
5. Open the public app's /alerts page. Choose a fictional area, confirm consent, enter the invitation and enable notifications using the button. Browser permission is separate from server enrollment.
6. Create a fictional missing-child report, prepare a draft for that same area, independently approve it and publish.
7. Observe the actual notification on the enrolled device. Record alert ID, device/browser, timestamp, provider result and observed display separately. Never record keys or private endpoint values in evidence.
8. Open the notification and verify current public status. Submit a fictional private tip, then withdraw the alert. Verify the current page removes the description and tip form.
9. Revoke the subscription from its original browser. Close the rehearsal and restore DISABLED mode when finished.

The test invitation is a bounded beta mechanism, not a production identity system. Clearing browser storage may lose the management capability; an operator recovery process remains required.

## Failures and stopping
Set NOTIFICATION_MODE=DISABLED for both API and worker and restart them to stop new sends. Withdraw active alerts to cancel queued work. An already accepted push cannot be recalled; an in-flight bounded request may finish before withdrawal commits. Never automatically replay AMBIGUOUS jobs.

If the worker is down, database receipts can still succeed while notification delivery stops. Watch queue age, worker liveness, error counts and unaccepted cases; production monitoring is not complete. Do not interpret an empty error log as proof of delivery.

## Before a real pilot
Name the issuer, on-duty responder and independent fallback; define acceptance deadlines, escalation acknowledgement, operating hours, incident contact and shutdown authority. Rehearse backup restore and data retention/legal holds. Require MFA and audited access. Government services are external resources, not connected dispatch destinations in this repository.
