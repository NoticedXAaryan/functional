# 02 · Child and community flows

All routes below are target routes. Existing state-based screens can be migrated gradually; URLs never contain narratives, return secrets or a child's name. Shared behaviour and error contracts apply to every flow.

## Navigation map

Home → Ask for help → Choose/record/type → Reply choice → Review → Received → Return setup → Conversation.

Home → Someone is missing → What you know → Place → Review → Private receipt.

Home → My messages → Three words + device proof, or return card → Conversation.

Home → Nearby alerts → Choose area → Read alert → Share a sighting; optional separate device notification enrollment.

“I am worried about someone” is a secondary help entry, not a different account type. Staff sign-in belongs in the footer. No child signup, email, phone OTP, Aadhaar, exact age or parent approval gate is added to initial help-seeking. Information required for a later intervention is requested by the responsible practitioner, with an explanation.

## Public screen contracts

| ID / target route | Content and controls | State change / next |
| --- | --- | --- |
| C01 `/` | “You can ask for help here.” Main **Ask for help**; secondary **My messages**, **Someone is missing**, **Nearby alerts**; language and Listen controls | No session until an action needs one; fetch public capabilities and operator availability |
| C02 `/help/start` | “What would you like help with?” Options **Something happened to me**, **Something online worries me**, **I am worried about someone**, **I am not sure** | Store selection in memory; selecting Not sure is valid; Next creates a private session with actual receiving team |
| C03 `/help/message` | “What would you like us to know?” **Type a message**, **Record my voice**, **Just ask for help** | At least a meaningful selected intent is enough; no minimum narrative; text/voice may coexist |
| C04 `/help/replies` | “Would you like messages here?” **Yes, when I come back**; **Send this without replies**. “We will not send notifications about this conversation.” | Explicit in-app permission; optional external contact not part of beta default |
| C05 `/help/review` | Summary of only the child's choices; play voice only on tap; each section has **Change**. “This goes to [actual team name].” **Send to the support team** | Idempotent create; no automatic public posting or parental notification |
| C06 `/help/received` | “Your message was received.” “This means it was saved for the team. They may not have read it yet.” **Choose how to come back**, **Leave this page** | Receipt shown only after durable commit; return setup must not delay first send |
| C07 `/help/return-setup` | “Choose three words to help you come back.” Picture-word cards, one at a time; **Different words**; shared-device explanation | Complete device or recovery-card flow in doc/04; child can decline without losing the submitted report |
| C08 `/my-messages` | “Open your messages.” Three word fields/cards with text alternatives; **Open my messages**, **Use my return card**, **I cannot get in** | Exchange device proof and words, or recovery credential; generic errors, no account enumeration |
| C09 `/conversation` | Actual support worker's approved display name and organization; accessible message timeline; Type/Record; **My reply choice**, **Leave this page** | Append messages, show durable state; no public URL with case ID; read-only when closed |
| C10 `/help/another-team` | “Is someone from this place involved?” **Yes**, **No**, **Not sure**, **Skip**; “You can ask for a different team.” | Yes/request selects independent routing; no names required; never return the report to the implicated venue |
| C11 `/help/options` | Verified help resources; 1098 child helpline and 112 immediate danger call links where applicable; **Back to my message** | Explain calling opens phone and can appear in call history; no automatic dial, location send or claim of integration |
| C12 `/privacy` | Short layered explanation of team access, device traces, optional voice, return cards, retention and contact | Read without session; Listen reads this public copy only after tap |

C10 is an optional “Ask for a different team” link throughout intake and conversation, not a compulsory trauma question. A QR venue supplies routing context only; it must never silently establish that venue's trustworthiness. A revoked QR shows “This link is no longer available” and offers the operator's verified general/independent help route if configured. Never silently substitute the local venue after an independent routing failure.

## Conversation details

Messages are visually labelled **You** or **Support worker · [approved name]**. Staff notes never appear. When assigned staff change, explain “Another support worker from [team] is helping with your messages.” Do not expose internal risk scores or speculative labels.

Allowed states: **Sending**, **Sent**, **Could not confirm**, **Read by the support team**. Read status is optional and requires an explicit server event. Assignment alone is not a read. An offline phone keeps the unsent draft in memory and offers Retry; it never silently uploads after Leave. Do not add typing indicators or read receipts exposing the child's presence to others in the first beta.

Show a truthful availability sentence from configuration: “You can send a message now. A support worker may reply later.” If actual coverage is unavailable, state it and show immediate-help choices. No guessed wait time or “24/7 support” badge. Children can open, read and send outside a rota; a rota determines staff escalation, not access.

Changing from no replies requires child confirmation. Turning replies off prevents new child-visible replies but retains messages already sent, with explanation. Keep an independent-team request available when the assigned responder is part of the concern. Closed conversation: show closure explanation and **Ask for help again**, preserving the old report; do not silently reopen it or imply the child's situation is resolved.

## Missing child: conversational intake

| ID | Screen | Required / optional and outcome |
| --- | --- | --- |
| M01 | “Who are you worried about?” **A child is missing**, **I saw a child who may need help**, **I am lost** | “I am lost” routes into child help with optional safe location; never creates a public location pin |
| M02 | “What do you know?” Type, voice or **I need someone to help me explain** | Approximate age, name, clothing, photo are optional at intake. No FIR upload or required identity proof |
| M03 | “Where were they last seen?” **Choose a place**, **Use my location**, **I do not know** | Warn current phone location may differ from last seen. Confirm landmark/area and approximate time; allow correction before send |
| M04 | “Check what you are sending” | Private review, intended team and optional contact method; **Send privately**. “The team will review this before sharing an alert.” |
| M05 | “Your message was received” | Private return path; immediate-help link; no “Alert sent” until a separate approved publication occurs |

Minimal reports are actionable requests for human follow-up, not automatic broadcast candidates. Public publication needs verified identifying/location information collected by staff. Unknown fields remain unknown; never infer a kidnapping from a missed return time. A bystander does not need to confront, follow or take photos of a child to send a concern.

## Community alert and sighting screens

| ID / route | UI | Outcome |
| --- | --- | --- |
| A01 `/alerts` | Nithari Alert + plain label; area search with results showing district/state; **Use my location** optional; list and text alternative to map | Reading needs no account and no notification permission |
| A02 `/alerts/{id}` | Current reviewed public details, approximate last-seen location/time, issuer and last updated time; **I saw something**, **Share this link** | Only ACTIVE unexpired alert exposes identifying details; share canonical link rather than generated poster |
| A03 `/alerts/{id}/sighting` | “What did you see?” Type/voice; “Where and roughly when?” optional guided selection; **Send privately** | Accept minimal observation; no identity/contact mandatory; instructions not to follow or confront |
| A04 Sighting receipt | “Your message was received by the reviewing team.” | Receipt is not review or police dispatch; optionally send another distinct observation |
| A05 `/alerts/notifications` | Area, explanation, **Allow notifications**, **Not now** | OS permission only after explicit action; separate from private support |
| A06 `/alerts/settings` | Saved area, **Change area**, **Stop notifications**, browser-specific blocked/unsupported state | Server revocation and browser unsubscribe tracked separately; no false success |

Expired, withdrawn or resolved links keep their route but remove identifying details, disable new sightings and explain “This alert has ended. Please do not share old copies.” Resolved does not disclose where the child now lives. If status cannot be fetched, hide stale identity details and offer Retry; do not display a cached active alert as current.

## Universal failure and exit behaviour

| Condition | Required response |
| --- | --- |
| Loading/cold backend | “Connecting to the support service…” then bounded timeout with Retry and immediate-help links; no endless spinner |
| Offline before send | “This has not been sent. Try again when you have internet.” Keep memory draft while page is open |
| Response lost after send | “We could not confirm it yet.” Retry same operation ID and payload; do not create another case |
| Server rejects | Short reason, focus relevant choice, retain draft; never expose SQL/provider traces |
| Voice denied/unavailable | Type and choice-only alternatives remain visible; no forced repeated permission prompt |
| Session expires | Hide private history, stop audio, offer return access; do not lose a draft silently or send it with another session |
| Intake closed | Explain online messages cannot currently be accepted; provide verified alternatives; do not show a dead submit button |
| Leave this page | Immediately stop microphone/playback/speech, discard unsent memory, revoke active access best effort, navigate to configured neutral HTTPS destination without confirmation |

Explain near the exit control through an optional help link: “This changes the page. It cannot erase browser history, downloads or call history.” Quick Exit is not a hidden-browser mode. Shared-device persistence is off unless explicitly chosen; no sensitive analytics, session replay, automatic clipboard writes or background draft caching.
