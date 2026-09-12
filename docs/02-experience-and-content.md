# Experience and content

## Current visual direction
Use light surfaces, soft green/blue/lavender/peach sections, dark readable text and solid buttons. No neon accents, glow effects or glass styling. The home screen asks “What would you like to do?” and presents four actions. At phone widths, cards stack and navigation uses two columns. Use plain action labels and explain disabled features in context. Keep necessary preview notices short; do not hide actual service limitations.
## Navigation
Public home offers Ask for help, Is this okay?, Check a report and Savera Alert. Assessment is optional. Receipt appears after successful submission. Community notification controls belong to the community journey; do not request notification permission during private intake.

Staff workspace offers Cases and Alert Review after sign-in. Show the role and organization context, empty-state instructions, permission explanations and actionable failures.

## Screen requirements
| Screen | User decision | Required states |
| --- | --- | --- |
| Home | Start a report, return or read alerts | Demo label, limits, privacy explanation |
| Intake | What happened and what contact is safe | Editing, review, saving, retry, saved; preserve input on failure |
| Receipt | Keep a private return capability | Saved versus accepted clearly distinguished |
| Return | Check status and messages | Invalid/expired access, no messages, replies; no private data persisted in URL |
| Community | Area and notification consent | Unsupported, denied, enabled, rotated, revoked; browsing without push |
| Alert | Read current status, send private tip | Active, expired, withdrawn, resolved; terminal detail removal |
| Staff review | Prepare, independently approve, publish, close | Role restrictions, expiry, verification reference, mode and delivery outcomes |

## Copy rules
Say “saved” for a successful database receipt, “accepted by the push provider” for HTTP acceptance and “seen on this device” only when observed. Never say a responder is helping merely because a report exists. Do not promise anonymity, guaranteed safety, erased browser history or immediate response.

“Incognito” alone is misleading: explain separately what is cleared locally and what is retained by the service. Current quick exit leaves the page; its credential lifecycle remains a blocker. Language preference is not proof of a translated UI. “Is this okay?” is explicitly simulated in this build.

## Small high-value additions
Next: show last-refreshed time; preserve unsent input on recoverable failures; scoped duplicate-submission protection; plain status descriptions; staff assignment call-to-action; safe-contact preference displayed before composing a reply; accessible inline error summaries. Avoid automatic guardian notification and public sighting maps.

## Accessibility acceptance
All actions keyboard-operable; visible focus; associated field labels; errors announced with alert/status semantics; no color-only status; 200% zoom and small-screen reflow; reduced motion; minimum useful touch target around 44px. Test real screen-reader flows after wiring is complete. No analytics or third-party session replay on private screens.
