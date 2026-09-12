# 05 · Nearby alerts and location

## Reach that the product can honestly provide

Nithari Alert sends browser push to **devices that opted in for the selected area**. It cannot contact every phone physically nearby, override OS notification settings or send a government cell broadcast. An installed native app could later support consented background location under platform rules; that is a separate product and permission review. SMS/WhatsApp/cell-broadcast integration is not included by changing the UI label.

Initial beta uses chosen areas and optional one-time location to select them. Copy: “Get alerts for this area,” not “We notify everyone nearby.” Visiting a different area does not automatically update a saved subscription. Display the saved area and last update, and let users change it.

## Recipient enrollment flow

1. Read current public alerts without any permissions.
2. Tap **Get alerts for my area**. Explain that public missing-child notifications may appear on the lock screen; keep this separate from private help.
3. Choose district/locality with a text search, or tap **Use my location once**. Ask browser geolocation permission only then.
4. Resolve location to supported areas. Show **[Locality], [District], [State]** and **Use this area**. Current position is an aid, not proof of residency or incident location.
5. If denied, timed out, inaccurate or outside coverage, offer manual selection. The map is optional. Never block reporting because GPS failed.
6. Show consent and **Allow notifications**. Request OS permission from that action. Respect the existing adult community enrollment policy; children asking for private help are not enrolled automatically.
7. Register browser subscription and server management credential. Success appears only after both are established. Explain **Alerts are on for [area]**, with **Change area** and **Stop notifications**.

WebKit documents iOS/iPadOS push for home-screen web apps from 16.4. Provide capability-specific Add to Home Screen instructions when needed, without a forced install wall. Android/desktop also require actual browser support and permission. Denied/unsupported devices can continue reading alerts. A browser permission grant without server registration is an incomplete setup, not “on.”

## Geographic model

Retain current `alert_areas` and area-ID enrollment for the first working slice. Add versioned `geography(MultiPolygon,4326)` boundaries to areas and `geometry_version`. An approved alert targets one or more reviewed areas or a reviewed centre/radius intersecting those areas. Use PostGIS geography for metre-based calculations, not raw latitude/longitude degrees.

Default product proposal: publisher chooses one supported locality; advanced radius targeting is off until actual boundaries and maximum broadcast size are configured. No fictional nationwide coverage. A radius input has operator-configured minimum/maximum and an explicit audience estimate. Large changes require a fresh approval.

`ST_Intersects` can map a reviewed alert region to saved subscription areas. It means potential relevance to an area, not confirmed recipient proximity. Deduplicate overlapping area matches by subscription ID. Store polygon version used for each published revision so audit and retries are consistent. At dispatch recheck active alert, expiry, subscription consent/revocation and live/test separation.

One-time coordinates should be converted to an area and discarded; do not keep raw location or travel history merely for enrollment. Avoid third-party map/search services receiving private reports or child-location text. An operator-maintained gazetteer is sufficient initially. If a geocoder is added, assess its data processing, request minimization and spending first.

## Publication and delivery state machine

`private report → staff assessment → draft public revision → independent approval → explicit activation → durable outbox → recipient jobs → provider outcome`

Any public update creates a new revision and requires approval. Same preparer cannot approve. Activation and enqueue are one transaction. Dispatch uses the approved payload hash/revision. Each subscription/revision/type has a unique job key. Bound retries, apply backoff with jitter and distinguish permanent expiration from transient provider failures. Recheck withdrawal immediately before sending; network races mean a push already accepted by a provider cannot be reliably recalled.

Worker needs continuous execution or an explicitly designed durable event-driven replacement. Free sleeping web hosts and once-daily cron are not substitutes for near-time delivery. Never keep a free service artificially awake and describe that as operational reliability.

## Notification content and opening

Title: **Nithari Alert**. Body: **A missing-child alert is active for your chosen area. Open for current details.** Do not include child names, photographs, exact whereabouts, abuse allegations or private messages in push payloads. A payload carries only the minimum alert ID, revision, expiry and mode necessary for safe handling.

Opening uses a locally constructed canonical `/alerts/{id}` link, not an arbitrary URL from a payload. Fetch current status before revealing identity details. Ended alert: “This alert has ended. Please do not share old copies.” Expired cached push is suppressed where possible. Remove displayed notifications after terminal state when the client is reachable; do not promise removal from every device or screenshot.

UI metrics must distinguish queued jobs, provider-accepted jobs, failures and known opens. None proves the child was located. No “X people saw this” unless an appropriately privacy-preserving observation supports that exact claim. Do not track subscribers' movements or publish recipient lists.

## Failures and operational controls

- Provider rejects expired endpoint: deactivate that subscription and ask for re-enrollment on next visit; preserve case services.
- Stopping notifications: revoke on server first, unsubscribe browser second; show partial result and retry if either fails. Revocation must work even when new enrollment is disabled.
- Area change: use authenticated subscription update with an idempotent operation; keep previous area until new server state is committed, then display the confirmed area.
- Invalid/withdrawn alert: reject new sightings with a readable status; never lose a valid previously received tip.
- Worker unavailable: staff see **Notification delivery is delayed** based on worker heartbeat/queue age; don't falsely say published means delivered.
- Abuse: independently verify requests; rate-limit submissions and sighting spam without forcing CAPTCHA on every child; staff review duplicate reports, never automatically merge confidential narratives across organizations.
- Separate controls: public intake, alert publication, notification dispatch and voice upload each have their own kill switch. Disabling one does not erase already received reports or prevent subscription revocation.

Browser push has no SMS-style per-message service charge in this design, but hosting, bandwidth, storage, abuse controls and continuous worker execution still consume resources. Do not promise unlimited free operation. If the budget cannot support prompt dispatch, describe only the capability actually operating and do not open an emergency-notification service under a false claim.
