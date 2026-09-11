# Product requirements document

Working product: Functional / Bal Suraksha · Version 1.0 · 11 September 2026

Status: target requirements for staged implementation; real-data operating approvals remain open. A prototype exists, but individual requirements need acceptance evidence before they are marked complete. See [implementation status](12-implementation-status.md). This document owns requirement IDs and intended behavior; the API contract owns exact field names.

## 1. Problem and outcome

A child or bystander may be unsure whether an interaction is concerning, fear who will learn about a report, struggle to explain the situation, or receive no visible response after asking for help. Separately, a missing-child case may need prompt, verified local awareness, while unverified publicity can expose a child or spread false information.

The product connects low-friction private entry, accountable professional response, and controlled geographic alerts. Success is the person reaching an appropriate next step, with information handled as explained. Engagement time, daily streaks and notification volume are not product objectives.

### Product promise

“You can ask for help, see who will receive your information, and understand what happens next.”

The product cannot promise absolute anonymity, guaranteed rescue, continuous monitoring, deletion of every trace, an AI verdict about another person, or delivery to every phone in a location.

## 2. Users and jobs

| Actor | Job | Product access |
| --- | --- | --- |
| Teen seeking help | Understand a concern or tell a safe responder | General guidance without login; approved personal-data path; own private case access |
| Concerned friend | Share a worry without having to prove it | Same intake, with clear distinction between what they saw and what they inferred |
| Reporting adult | Request help about a child or missing-child concern | Their report and permitted progress; no entitlement to a child's separate case |
| Responder | Accept, assess, communicate and follow up | Explicit case grants in their permitted organization/jurisdiction |
| Safeguarding supervisor | Resolve unowned, urgent or conflicted cases | Authorized queue supervision, reassignment and review |
| Alert preparer / approver | Decide what verified information can be public | Separate, bounded publication roles |
| Adult subscriber | Follow chosen areas and provide useful observations | Approved public alerts; private tip submission |
| Venue administrator | Maintain verified QR placement and support availability | Placement management and privacy-preserving service aggregates |
| Platform operator | Keep the application reliable | Operational access by default; exceptional content access controlled and audited |

Initial usability scope is a teen journey, approximately ages 13–17, with adult reporting about younger children. This is not a consent-law exemption. P4 age handling and lawful processing require a documented decision. P0–P3 use adult developers/testers and fictional scenarios only.

## 3. Goals, exclusions and boundaries

### Goals

- Make the first useful action available from a QR without an installation or sign-up wall.
- Let the user understand recipients, contact safety and progress.
- Make every active support case someone's explicit responsibility.
- Give uncertain users bounded, understandable text-assessment assistance.
- Publish only the approved information in eligible, verified local alerts.
- Make notification readiness, failure, expiry and withdrawal visible and testable.
- Fit a small-budget staged build and preserve a path to a staffed beta.

### Excluded from P0–P3

Live child disclosures; autonomous emergency dispatch; official cell broadcast; government database access without agreement; background reading of messaging apps; continuous child GPS; facial recognition; public sightings maps; crowd pursuit; digital-fire-drill consumer games; SMS/WhatsApp campaigns; paid inference in the child flow; custom ML training; native mobile app; a general-purpose AI companion; and collection of intimate child imagery.

Deferred does not mean silently mocked. Demo screens and evidence must identify fixtures and simulated handoffs. Core report persistence, return access, staff messages, alert revision/approval and test push should be implemented as real application behavior in the relevant phase.

## 4. End-to-end journeys

### J1 — Unsure, then asks for help

QR → verified placement and language → “Is this okay?” → short explanation of handling → selected text or fictional demo scenario → bounded explanation → optional reviewed handoff → preview recipient and content → durable receipt → responder acceptance → conversation → follow-up.

Assessment failure leaves the human-help path usable. A high model score never publishes anything or determines statutory reporting by itself.

### J2 — One-time visit

QR → “Incognito — one-time session” → understand advice-only versus submitted report → use the session → exit. If the person submits for help, explain retention and separately offer return access. Ending access and erasing legally held records are different operations.

### J3 — School or home may be unsafe

Intake → safe-contact question and “someone here is involved” → independent route preview → limited disclosure to the correct responder → safe progress. Do not automatically notify the implicated institution or caregiver.

### J4 — Missing-child report and verified alert

Private report → urgent professional routing → incident verification → public fields and geographic audience prepared → separate approval → publish → test/live audience according to environment → restricted tips → review → updated, withdrawn or resolved notice. The initial request for help does not depend on public-alert eligibility or possession of an official reference.

### J5 — Subscriber receives and acts

Choose areas → explain notifications → request OS permission after a user action → register subscription → send readiness test → view current alert → submit what/when/where privately → receive submission acknowledgement. No public feed of exact sightings or personal tip content.

## 5. Requirement catalog

Priority: **Must** = necessary for that phase's stated outcome; **Should** = useful and cuttable before a Must; **Later** = deliberately deferred. A phase field is the earliest planned delivery. P4 requirements are never prerequisites to demonstrating synthetic P1–P3.

### Quiet Door

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| QD-01 | Must P1 | Resolve a QR placement to verified name, language and routes. Unknown/revoked placement shows a safe generic entry and no false venue endorsement. |
| QD-02 | Must P1 | Offer the four entry choices without sign-up. General guidance and urgent contact information remain accessible when optional processing is unavailable. |
| QD-03 | Must P1 | Show recipient and handling preview before detailed submission. Changing route updates the displayed recipient before confirmation. |
| QD-04 | Must P1 | “Someone here is involved” selects an independent route. The implicated venue has no case grant merely through placement affiliation. |
| QD-05 | Must P1 | Show editing, submitting, received and failure states truthfully. A timeout permits retry with the same idempotency key and produces one case. |
| QD-06 | Must P1 | Let users skip optional fields and choose “I'm not sure.” Keep observation separate from inference; do not require a legal category or proof. |
| QD-07 | Should P1 | Provide quick exit, language switching and a shared-device warning. Exit invalidates active access; browser-history limitations remain explicit. |

### Incognito and continuing access

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| IC-01 | Must P2 | Label “Incognito — one-time session.” State no account/no continuing product history, with actual processing/retention notice; never claim zero traces. |
| IC-02 | Must P1 | Separate check-and-leave from send-for-help. Case creation needs an explicit submission or the disclosed required safeguarding pathway, not an ordinary model response. |
| IC-03 | Must P1 | Optional continuing access uses an unguessable secret. Failed submission retries remain recoverable; secrets never enter query strings, analytics or logs. |
| IC-04 | Must P2 | Exit/inactivity invalidates session credentials and clears private local state. Private responses are not stored by the PWA. Browser-back is tested on supported devices. |
| IC-05 | Must P2 | Explain lost-code and one-time limitations before leaving. Do not recover a case by matching a name or send recovery information to an unapproved contact. |

### Is this okay?

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| AI-01 | Must P2 | Accept voluntary text within a documented size limit. P2 defaults to clearly labeled fixture responses; unsupported inputs show an honest fallback. |
| AI-02 | Must P2 | Output observed behaviors, references to supplied text, uncertainty and safe options. Never certify a person safe, diagnose or present an unvalidated numerical risk score. |
| AI-03 | Must P2 | AI failure, timeout, quota or unsupported language keeps reporting usable. No automatic fallback to an unapproved external provider. |
| AI-04 | Must P4 | Live provider deployment requires child-use eligibility, processing/retention assessment, bounded configuration, reviewed evaluation and rollback. Developer use of Gemini does not satisfy this. |
| AI-05 | Must P2 | Input is untrusted data. Embedded instructions cannot trigger tools, reveal secrets, change recipients, create public alerts or contact third parties. |

### Support cases

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| CS-01 | Must P1 | A receipt is issued only after durable commit. Restart/retry tests preserve one received case and its access behavior. |
| CS-02 | Must P1 | Each case can be triaged, assigned and accepted by an authorized actor. Assignment and acceptance are visibly distinct. |
| CS-03 | Must P1 | Case messages are scoped to permitted participants, ordered/paginated and duplicate-safe. Child-visible messages are separate from restricted professional notes. |
| CS-04 | Must P1 | Record safe-contact preferences. A risky contact flag prevents default external notifications or referral to the implicated party. |
| CS-05 | Must P1 | Unaccepted cases show a due time and supervisor escalation. Timers do not depend on AI or an operator leaving a browser open. |
| CS-06 | Must P4 | Referrals record receiving party, lawful sharing basis, acknowledgement and next review. “Exported” does not mean the receiving service acted. |
| CS-07 | Must P1 | “I still need help” creates a visible reopen/review request; authorized staff reopen with a reason. The child's request is not a silent state change or lost message. |
| CS-08 | Must P4 | Staff offboarding revokes access and reassigns active cases. Closure records a reason; public alert resolution does not automatically close safeguarding work. |

### Verified local alerts

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| AL-01 | Must P3 | Missing-child concerns enter private routing immediately. An official reference is optional at intake; publication eligibility is separately assessed. |
| AL-02 | Must P3 | Draft, review and approval roles are enforced by API authorization. The preparer cannot approve their own revision in the normal workflow. |
| AL-03 | Must P3 | Approval binds exact text, photo, issuer, area IDs/geometry version, expiry and tip destination. Editing any bound field invalidates old approval. |
| AL-04 | Must P3 | Publish writes state and outbox work atomically. Concurrent/retried publish produces one logical revision event. |
| AL-05 | Must P3 | Public responses contain only approved fields, with current status, issue time, expiry and issuer. Private case IDs/content and reporter contacts do not leak. |
| AL-06 | Must P3 | Authorized withdrawal/resolution stops future jobs and issues appropriate updates. Already displayed notices cannot be promised recall. |
| AL-07 | Must P3 | Sightings remain private; capture what/when/where and optional safe callback. No direct exposure to the claimant or public map. |
| AL-08 | Should P3 | “Report a problem with this alert” enters a private moderation queue as an alert issue. It never posts a public accusation or auto-removes a notice. |

### Notifications and community

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| NT-01 | Must P3 | Adults explicitly choose areas and language. Explain that areas are followed places, not detected current location; no background GPS by default. |
| NT-02 | Must P3 | Request notification permission only after a user action and explanation. Permission denial leaves the public current-status page usable. |
| NT-03 | Must P3 | Readiness panel distinguishes unsupported, permission denied, subscribed and test-message states. “Test sent” means provider acceptance unless receipt is observed. |
| NT-04 | Must P3 | Match approved area versions and deduplicate per recipient/channel across overlaps. Retry rechecks active revision, consent and expiry. |
| NT-05 | Must P3 | Unsubscribe/revocation cancels future eligible jobs; stale/broken endpoints are disabled. Changing areas does not change child-session identity. |
| NT-06 | Must P3 | Demo sends require TEST_ALLOWLIST and visibly fictional notices. No device is enrolled by scanning a QR; no uncontrolled bulk send or paid SMS fallback. |

### Operations

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| OP-01 | Must P1 | Service hours and available routes are visible. Server receipt is never presented as a live human response or guaranteed response deadline. |
| OP-02 | Must P4 | Verified partner, duty coverage, supervisor and escalation destination exist before real intake. Missing coverage has an explicit service fallback. |
| OP-03 | Must P1 | Record content-free access/action audit metadata sufficient for investigation; no secrets or narrative text in ordinary logs. |
| OP-04 | Must P3 | Feature switches separately disable assessment, publication and delivery. Disabling one does not destroy received cases. |
| OP-05 | Must P4 | Retention, legal holds, privacy requests, breach response and backup restoration have named owners and exercised procedures. |
| OP-06 | Must P4 | Publication authority, safe identification rules and official handoff are documented. Beta does not claim live government APIs or official broadcasting without access. |

### Nonfunctional requirements

| ID | Priority / phase | Requirement and acceptance |
| --- | --- | --- |
| NFR-01 | Must P1 | Object-level and organization-level authorization on every case, message, attachment and action; cross-tenant tests fail closed. |
| NFR-02 | Must P1 | Maintainable Go/React/TypeScript implementation uses the common API contract; strict validation and predictable errors at boundaries. |
| NFR-03 | Must P1 | Keyboard usable, legible at 200% text zoom and narrow mobile widths; P4 targets WCAG 2.2 AA with human testing. |
| NFR-04 | Must P1 | Client supports timeouts/retries without false receipts or unapproved durable storage of drafts. Core routes work on agreed low-bandwidth devices. |
| NFR-05 | Must P3 | Queue operations recover after worker crash, deduplicate retries and enforce expiration; delivery metrics distinguish states. |
| NFR-06 | Must P0 | Mode validation prevents demo credentials, fixture labels removed falsely, TEST recipients or stub handoffs from becoming production behavior. |
| NFR-07 | Must P2 | Provider requests have timeouts, size/output bounds and hard application caps; costs cannot silently expand through retry or fallback. |
| NFR-08 | Must P4 | Restore, migration rollback/forward recovery, staff MFA, incident access and secret rotation are tested before real-data launch. |

## 6. Small features selected by failure point

| Failure point | Small feature | Benefit | Requirement |
| --- | --- | --- | --- |
| User retries after a timeout | Receipt-aware retry with one submission key | Avoids duplicate reports and uncertainty | QD-05, CS-01 |
| User fears the wrong person will see it | Recipient preview and independent-route switch | Makes information flow understandable | QD-03, QD-04 |
| A phone is shared or monitored | Safe-contact preference and quick exit | Reduces accidental disclosure | CS-04, IC-04 |
| A person cannot classify the incident | “I'm not sure” and optional fields | Removes an unnecessary barrier | QD-06 |
| A case appears closed too soon | “I still need help” | Makes unresolved needs visible | CS-07 |
| Notification setup silently fails | Readiness panel and test message | Establishes what actually works on the device | NT-03 |
| A forwarded alert is outdated | Current-status link with expiry | Helps recipients verify relevance | AL-05, AL-06 |
| A notice is wrong or suspicious | Private alert-issue report | Gives moderators a correction route | AL-08 |

## 7. Measures and provisional quality targets

Track durable receipt latency, acceptance delay, unaccepted case age, missed review times, acknowledged referrals and unresolved follow-ups. Collect optional task-completion/usability feedback without session replay or capturing private content in analytics. Suppress small groups in organization reports.

For alerts, track eligible recipients, jobs created, provider acceptance, observable delivery, opens where supported and reviewed useful tips. Do not equate provider acceptance with a person being informed or a child being protected.

Provisional engineering targets: under agreed test load, p95 non-AI receipt within two seconds and notification jobs created within one minute of approval. Beta availability target and recovery objectives must be agreed and funded; a 99.9% aspiration is not an existing SLA. Human response targets come from staffing, not a UI timer.

## 8. Dependencies and release conditions

P0–P3 need development devices, local database/runtime, fictional fixtures, an adult test audience and a supported push environment. A smartphone accessing a laptop's plain HTTP LAN address may not have the secure context needed for push: use the documented HTTPS test path or demonstrate on localhost and disclose the limitation.

P4 additionally needs an accountable partner, lawful processing/retention decisions, age-appropriate notices, incident ownership, response coverage, safe publication authorization, privacy-reviewed vendors and security evidence. AI provider eligibility and public-alert operational eligibility are independent gates.

## 9. Definition of done and change control

A feature is done only when its user journey, API behavior, persistence, negative paths, permissions and relevant tests work in its stated mode. Screens backed by fixtures must say so. Evidence identifies the commit, environment, dataset and what was actually exercised. Documentation-only work never marks application requirements implemented.

Resolve contradictory defaults in [decisions](10-decisions-and-sources.md). Preserve requirement IDs when behavior changes; record a superseding decision and update the API, UX, test mapping and phase scope together. Scope cuts defer whole capabilities explicitly rather than removing their access controls or truthful failure states.
