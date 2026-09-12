# 03 · Responder and operator flows

Staff are accountable named people from configured organizations. The UI does not imply that a login alone makes someone a trained safeguarding responder. Provisioning, training and independent escalation are operator responsibilities.

| ID / screen | Primary task | Controls, state and permissions |
| --- | --- | --- |
| S01 Sign in | Restore or establish authorized staff access | Organization sign-in or deployed maintained session flow; real accounts only; show password, accessible errors, supported recovery, MFA through chosen provider |
| S02 My work | Find the next request needing attention | Assigned/granted cases only for responders; supervisors see own organization queue; New, Waiting for me, Waiting for child, Escalated filters; no public narrative in notifications |
| S03 Request detail | Understand what the child chose to share | Original text/voice, chosen language, permitted reply mode, routing conflicts and actual timeline; no auto-play; no required transcript |
| S04 Assign | Give a real responder responsibility | Supervisor picks active eligible person; preview name/team; atomic assignment/version update; confirm state only after commit |
| S05 Accept | Acknowledge responsibility | Assigned responder explicitly accepts; competing reassignment yields refreshed state, not a silent overwrite |
| S06 Conversation | Reply in text or voice | **Reply to child** and **Private team note** are separate labelled modes; voice note gets a typed summary before child-visible send; review recipient and format |
| S07 Transfer/escalate | Obtain help from another authorized person/team | Reason, minimum necessary summary, accepting team, delivery and acknowledgment; ownership remains until receiver accepts |
| S08 Close | Record agreed outcome and remaining options | Reason and child-visible message; no-contact cases keep message private; closure is not a claim that abuse has stopped; child can make a new request |
| S09 Private sightings | Review community observations | Approver/supervisor authorized for issuing organization; Listen, mark reviewed, assign follow-up; sighting never appears in public feed |
| S10 Prepare alert | Produce minimum necessary public information | Distinct from case notes; public preview, issuer, location geometry, duration, supporting verification and risk assessment; submit for independent review |
| S11 Review alert | Decide whether publication is justified | Different eligible approver; approve/reject with reason; immutable approved revision; modification invalidates approval |
| S12 Publish/manage | Activate or end approved alert | Explicit recipient area/count estimate, expiry, **Publish alert**; withdraw/resolve available to authorized staff without a second approval that would prolong harm |
| S13 Service settings | Keep the service honest | Actual operator details, intake availability, independent routing, staffing coverage, verified help links, supported languages and feature flags |
| S14 Staff/area administration | Maintain access and coverage | Role grant/revoke with audit, organization active state, entry-point revoke, geographic area version; destructive changes require explanation |
| S15 Sign out / expired access | End private access | Revoke server session, clear private memory/media; failed revocation reported without claiming remote access is gone |

## No rigid schedules

Remove the current default 09:00–17:00 requirement. A child can open the service and send a request at any hour when intake is available. A responder can leave an in-app reply at any hour when the child opted into replies. No private push/SMS/call is triggered by an in-app message.

Separate three concepts in storage and UI: **permission for in-app replies**, **permission for external contact**, and **staff coverage**. Quiet hours, if external channels are introduced later, apply only to those channels. Existing no-contact and time-limited permissions must not be broadened automatically. Ask the child to update their choice on return; until then preserve the existing stricter permission. Staff may record private notes outside that window, but cannot circumvent it by labelling an outward reply as a note.

Allow draft replies at any time. Future external-contact drafts can be queued for a child-chosen safe window, clearly marked not sent. Never tell staff to bypass consent to improve a response-time metric. Safeguarding exceptions require a documented, authorized operator procedure and an audit trail, not an arbitrary UI override.

## Responder voice and listening

Both child and responder voice use the same recording/playback component and private attachment pipeline. Before playback, show “Sound may be heard by people nearby” and explicit Play. Offer pause/stop, seek, elapsed time and playback speed. Browser/OS controls where sound goes; do not claim the app can reliably select speaker versus earpiece. Headphones are a suggestion, not a gate.

Responder-written messages have optional **Listen** only with a supported local voice or approved private speech service. Child-facing voice replies require a short responder-written equivalent so they can be understood without sound. Do not force the child to type a transcript of their own recording. A staff-written summary of a child's audio is marked “Support worker's summary,” separately from the original.

## Assignment, escalation and continuity

Queue status is derived from actual events, not optimistic labels. Pending assignment means no named person has accepted it. Escalation has `created`, `notified`, `acknowledged`, `accepted`, `failed` states; a worker log is not delivery. Supervisor dashboard makes unacknowledged and failed escalations visible with an alternative contact route. Thresholds and escalation targets are operator-configured, audited and based on actual coverage; do not fabricate a response SLA.

When a child flags the venue/team as unsafe, remove that team's access and route to a configured independent organization through a reviewed policy. If none exists, explain the service cannot route privately there and offer verified alternatives before collection; never present the same team under another label. Revoke grants and media access on reassignment where required. A past cached role must not keep authorization after offboarding.

## Public alert publication safety

No public alert can be produced simply by a distressed reporter pressing Send. Require an authorized issuer, independently reviewed information, minimal identity fields, verified location scope, bounded expiry and a publication rationale. Check abuse/custody/trafficking risks before displaying a child's identity or location. An alleged guardian's request is not sufficient authority.

The alert reviewer sees exactly the public card and notification text that recipients will see, plus separate private verification information. No alleged offender accusation, home address, phone number, shelter location, private chat or audio in public payloads. Verify the basis for any public photograph; do not scrape images. Do not append medical or abuse history.

Staff send action copy: **Publish alert to [area]**. Confirmation: “This will make the reviewed details public and notify subscribed devices for this area.” A bounded estimate is labelled estimate, not exact delivery count. Audit who prepared, approved, activated, amended and ended every revision.
