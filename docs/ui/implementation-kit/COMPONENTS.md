# Component contracts: implement exactly, then connect real services

## File placement

Use the existing React/TypeScript apps. Do not migrate frameworks or add a second router without inspecting current navigation. Suggested organization: `src/ui/` for shared presentation, `src/features/help/`, `src/features/return/`, `src/features/voice/`, `src/features/alerts/` for domain screens, `src/api/` for typed transport, `src/content/` for catalogs. A shared workspace package is optional only after the existing build supports it. Do not break independent Vercel root-directory builds by importing files outside their available build context.

Import `tokens.css` once. Wrap only migrated screens in `.bs-ui`; do not globally rewrite unrelated screens in the same task. Remove conflicting old styles from a migrated screen, rather than stacking new overrides on neon/glass classes.

## Supplied React primitives

`Primitives.tsx` contains ActionButton, ChoiceButton, ChildShell, InlineError and VoicePlayer. They are actual presentational React definitions, but have not been built within either app. They intentionally do not implement API calls, auth, recording, private blob acquisition or navigation history.

- `ActionButton`: set `type="submit"` only inside the owning form. Show `disabledReason` while a required choice is missing. Pending action needs an accessible status message supplied by parent, not only changing colour.
- `ChoiceButton`: `onChoose` must update one controlled value; no send side effect. For a single selection, use a labelled native radio group or implement equivalent keyboard semantics. The primitive's pressed-state buttons alone are not a complete radio-group implementation.
- `ChildShell`: `onLeave` must call the complete exit procedure below. `languageControl` and footer are required content, not empty placeholders. Add skip link and heading focus management when integrating routing.
- `InlineError`: translate stable server codes; don't pass arbitrary server error text containing personal or technical data. `onRetry` reuses the previous operation where relevant.
- `VoicePlayer`: use an authorized same-origin stream only with appropriate cookies. For the existing child bearer flow, fetch with bearer authorization, create a short-lived in-memory Blob URL, pass it as source and revoke it on teardown; a bare audio URL cannot attach a bearer header. Never put a bearer token into an audio URL. The caller owns stopping every player on exit and permission changes.

## Components still to implement

| Component | Required inputs | Events | Must never do |
| --- | --- | --- | --- |
| StepHeading | heading, description, optional step label | focus after navigation | Use a false fixed step count when branches change |
| MessageComposer | draft text, ready attachments, permissions, submitting state | textChanged, recordRequested, attachmentRemoved, submit | Submit on typing or create fake message IDs |
| VoiceRecorder | limits, availability, labels | recordedBlob, cancel, error | Record before tap; use a microphone denied state as a reason to block text |
| ReturnKeyCards | three controlled word IDs, reviewed vocabulary, locale | selectSlot, replaceSuggestion, confirm | Persist words automatically or search all cases by phrase |
| ReceiptStatus | actual receipt, current read/assignment state | returnSetupRequested | Show “received” before committed response |
| ServiceStatus | actual intake/coverage/capability state | retry, externalHelpRequested | Derive staff presence from a role count |
| LocationChoice | actual supported areas, selected area, optional transient coordinates | manualAreaSelected, locationRequested, locationConfirmed | Assume current device location is incident location |
| AlertCard | active approved current public alert or terminal state | open, shareCanonicalLink | Cache private/expired identity details |
| ConfirmAction | exact action, consequences, submitting state | confirm, cancel | Trap focus without escape/cancel; hide destructive consequence |

## State ownership

App root owns current language, capability configuration, navigation and active auth context. Help flow owns one in-memory draft and one stable submission operation. Recorder owns temporary media until it returns a blob. API layer owns parsing and mapping response contracts, not UI messages. Server owns authorization, committed status, storage associations, routing and delivery states.

On language change, only presentation changes. On navigating Back, preserve memory draft. On Leave, erase memory. On auth expiry, hide old private history and prevent draft submission until the same case/session is re-established. Do not attach a preserved draft to a different case.

## Complete exit sequence

1. Immediately stop all MediaStream tracks, audio/video players and active speech synthesis.
2. Mark active flow cancelled and abort in-flight requests using AbortController. A previously accepted server write may still have committed; do not promise deletion.
3. Clear draft, selected case, message history, return words and active bearer from memory; revoke object URLs and remove event subscriptions/timers.
4. Request active credential revocation best effort with keepalive where supported. Never wait for it before navigating.
5. Replace location with a configured neutral HTTPS destination. Explain that this does not erase history/downloads/call logs.
6. On pageshow from back-forward cache, clear private state/reload. A deliberately saved return-device cookie is separate; Forget this device revokes it explicitly.

## Exact status mapping

| Server/transport fact | UI label | UI behaviour |
| --- | --- | --- |
| Request not yet sent | Unsent | Keep draft editable |
| Request in flight | Sending… | Disable duplicate send, allow Leave |
| Durable commit returned | Sent / Your message was received | Show actual receipt; clear submitted draft |
| Connection lost with unknown outcome | We could not confirm it yet | Retry same operation; don't claim failure or success |
| Explicit staff read event | Read by the support team | Optional display; not inferred from assignment |
| Alert outbox committed | Alert published; notifications queued | Distinguish publication from provider outcome |
| Provider accepted push | Accepted by notification service | Never call this device delivery |
| Alert terminal state | This alert has ended | Remove identifying details and new-tip form |

## Data rendering rules

Render case and tip content as text, not HTML/Markdown with arbitrary links. Never use dangerouslySetInnerHTML for reports. Preserve line breaks. Bound displayed names and narratives without cutting off access to the full authorized text. Source organization names and responder display names from authorized API fields. Escape search input and validate URLs before navigation. Raw identifiers can appear in an expandable staff detail panel, not as the child's primary heading.

No audio is embedded in public alerts. No real child images are included in this kit. Audio files, return cards and event receipts must be generated from the actual user's authorized flow, not copied from a static resource.
