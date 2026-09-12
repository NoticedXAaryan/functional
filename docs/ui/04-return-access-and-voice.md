# 04 · Return access and voice

## Three words: experience and threat model

The child sees three simple words with optional pictures, for example **mango · cloud · boat**. These are illustrative copy, never a seeded credential. Allow selecting one word from each reviewed group, replacing suggestions, typing and changing language. Avoid sensitive/religious stereotypes, names, homophones and words likely to distress children. Pictures supplement text and accessible labels.

Three words selected from a 2,048-word list have at most 33 bits of entropy even when uniformly random; child-chosen combinations are usually weaker. Encrypting or hashing them does not make them equivalent to a random 256-bit key. They must **not** be the sole global credential for a private abuse report, and the UI must not imply they work alone on every device.

### Chosen design

1. On submission, keep the existing high-entropy return capability. Do not change legacy access simply to improve wording.
2. After receipt, offer **Use this device again** or **Use a return card**. Explain “Only choose this device if it is safe to keep a way back here.” Neither choice is mandatory for submitting.
3. Device option: create an independent 256-bit credential, stored in a Secure, HttpOnly, same-origin cookie after explicit opt-in. Store only its keyed digest server-side. Bind it to one case and one three-word alias verifier. Words are additional friction on that device, not a promise of protection against someone who controls the device.
4. On return, the device credential locates the record; the three words are checked within that record. No global search for a phrase, no exposing collisions or registered aliases. The UI never reveals remembered words on the locked screen.
5. Return-card option: an explicit download/print/share action packages a separate 256-bit recovery secret in a QR/card; words label it but do not replace it. Explain possession can open the messages and downloaded files can remain on the device. Default: no download, clipboard write, device persistence or OS share sheet.
6. On another device, use the card or a deliberately saved secure code. Do not offer three words alone. If neither is available, explain that private history cannot be recovered this way; offer a new help request and practitioner-assisted continuation without disclosing old history based on a guessed narrative.
7. After authenticated recovery, offer revoke-other-devices and rotate the recovery card. Rotation is explicit and invalidates the old credential atomically; never destroy the only working credential before a replacement is acknowledged.

Microcopy: “Your words work with the private key saved on this device. On a different device, you will need your return card.” For shared devices prefer one-time use, no persistence. Let children decline a return method without pressure.

### Implementation requirements

- Use versioned vetted wordlists with stable word IDs. Store ordered canonical IDs and language/version metadata; translating visible labels does not change the key. Normalize supported script/case/spacing deliberately; do not silently accept arbitrary fuzzy spellings.
- Store an Argon2id verifier of the canonical phrase with unique salt using an established library; use a server-held pepper if adopted with a documented rotation path. Never encrypt case content directly with three words.
- Device/recovery credentials use cryptographic randomness, separate purpose-scoped keyed digests, expiry and revocation. Secrets are not analytics events, URLs in logs, database plaintext or public receipt IDs.
- Start with existing 30-day return lifetime and 15-minute active access, visibly explained. Device persistence is capped by return lifetime. Longer cases need explicit supported renewal, not accidental permanent access.
- Rate-limit by credential and network with shared durable counters. Initial policy: five failed phrase attempts in 15 minutes per device, then cooldown; cap network bursts separately without permanently excluding a shared school connection. Counters never modify unrelated sessions. Offer recovery during cooldown; no global account lockout.
- Apply Origin/CSRF checks to cookie-backed mutation and access exchanges. Use constant-time secret comparisons and generic invalid/expired/revoked responses. Do not put child sessions in localStorage.
- Quick Exit revokes active access and stops media immediately. Saved-device proof remains only if the child explicitly chose persistence; **Forget this device** revokes it separately. Explain this distinction. A same-device attacker or browser malware remains a risk; no “fully protected” claim.
- Recovery QR parsing stays local and exchanges the secret by POST. If a fragment link is supported, remove the fragment before loading optional resources and never log it. Never accept recovery via query string.
- Roll out additively: existing 256-bit codes remain accepted until their current expiry; setup can be offered after successful old-code login. Do not revive retired four-word credentials or overwrite old hashes.

## Voice capture: one shared state machine

`idle → requesting_permission → recording → stopped_preview → uploading → processing → ready_to_send → committed`

Alternative states: `permission_denied`, `unsupported`, `interrupted`, `too_large`, `upload_failed`, `processing_failed`, `cancelled`. Upload completion is not message submission.

| State | User controls and exact behaviour |
| --- | --- |
| Idle | **Record my voice**, **Type instead**, **Choose buttons instead**; no permission requested yet |
| Permission | “Your microphone will be used only while you record.” OS prompt after tap; never automatically retry a denial |
| Recording | Visible timer and “Recording”; **Stop**, **Cancel**; tap controls, not press-and-hold only |
| Preview | **Play**, **Record again**, **Remove**, **Use this recording**; silence until Play |
| Upload | Visible progress where available, **Cancel**; preserve memory blob for retry; don't claim sent |
| Processing | “Preparing your recording…”; bounded failure path; original text/choices remain available |
| Ready | Attached duration and optional child text; final **Send** still required |
| Committed | Render a voice bubble; message ID and server state determine Sent |

Initial limits: 120 seconds and 8 MiB per clip, maximum three clips per message. Display limits before recording and stop at the limit without discarding the captured clip. Support a follow-up clip. These are proposed product limits, not measured hosting capacity. Enforce both client and server. Use `MediaRecorder.isTypeSupported` to negotiate WebM/Opus or MP4/AAC as actually supported; never assume all phones produce one format.

Stop media tracks and revoke object URLs on cancellation, unmount, session expiry and Quick Exit. Page hiding stops recording/playback and keeps an unsent preview in memory if possible. A phone call interruption yields a reviewable partial clip, never a silently sent recording. Remove discards unsent objects and schedules orphan cleanup if upload already occurred.

## Listening and accessible equivalents

No autoplay, including staff screens. Every Play/Listen action carries an accessible name. Display “Sound may be heard nearby”; no blocking confirmation on every play. Do not assert speaker routing control the browser does not provide. Support pause/stop/seek and text equivalents; audio controls work by keyboard.

For reading private text aloud, prefer a locally supplied speech voice where `localService` is true. If no suitable language voice is available, retain text and explain Listen is unavailable. Do not silently send a private message to a cloud speech service. Public static instructions can use reviewed bundled recordings for predictable language pronunciation. Private messages require a separately approved processor before cloud synthesis/transcription can be enabled.

Responder voice replies require a concise typed equivalent before sending. Child recordings do not require typing. Automatic transcription is off for initial beta; authorized human summaries are labelled, editable with audit, and do not replace original audio. No speaker identification, emotion detection, credibility scoring or voice biometrics.

## Private attachment pipeline

Authorize an upload intent against an active private session or authorized staff case before issuing a short-lived upload capability. Store in private object storage, never a public Vercel asset directory or a database base64 field. Bind upload ID, owner/case, byte limit, allowed format, checksum and expiry. The browser cannot choose a public bucket/key.

Finalize validates actual byte signature, duration, size and decode safety in a resource-limited process; reject malformed/polyglot files and strip nonessential metadata from the playback derivative. Use encrypted storage and transport with server-side access control; do not call this end-to-end encryption because authorized responders and processing services can access content. Preserve an original only under the approved evidence/retention policy, with separate access and audit.

Attachment lifecycle: `pending → uploaded → processing → ready → attached → deleted/quarantined`. Only ready attachments can be committed into a message; transaction checks case ownership and single intended association. Failed sends reuse the same operation and attachment IDs. Orphan objects expire after 24 hours unless a shorter approved policy applies. Submitted audio follows case retention/legal-hold rules; no arbitrary auto-delete that destroys evidence.

Private playback uses an authenticated stream endpoint with fresh case permission checks, `Cache-Control: no-store`, validated Range handling and no shared CDN caching. Stop new reads after revocation. Already delivered bytes cannot be recalled; say so in security documentation. Avoid long-lived bearer media links. Logs contain event IDs and durations/bytes, never voice, transcript, signed URLs or original filenames with personal data.
