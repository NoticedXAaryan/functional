# 01 · Brand, language and components

## Naming and incident connection

Use **Bal Setu** for the platform. Use **Nithari Alert** for the community missing-child feature, with the explanatory label **Nearby missing-child alerts** on every entry point. Nithari is an Indian missing-children incident documented by NHRC; its concern about missing reports and effective response provides the design rationale. This is not an official memorial, family endorsement, police affiliation or guarantee of rescue. Do not name an individual child or use victim images. Public historical explanation, if included, belongs on an optional adult-facing About page and must be factual, brief and non-graphic.

The incident name is a product proposal for implementation, not an assertion that affected families approve it. Before launching the public incident story, obtain review from the responsible operator and affected-community representatives where feasible. If they object, use **Setu Alert**; keep the descriptive label unchanged. Children should not have to learn the incident to use the service.

| Existing name/surface | Target | Compatibility rule |
| --- | --- | --- |
| Bal Suraksha / product titles | Bal Setu | Update visible copy, metadata, app manifest and documentation entry points |
| Savera Alert / Amber Alert | Nithari Alert | Update public and staff UI, push titles, operator material and QR landing pages together |
| `/alerts` and alert UUID URLs | Keep unchanged | Printed links and existing notifications must still resolve |
| `savera.subscription.v1` | Internal legacy key retained initially | Never discard subscription management credentials during a cosmetic rename |
| `/savera.svg` | New approved asset plus legacy alias | Old service workers and installed apps must not lose icons |
| Database names, enum values, IDs, Go module paths | Keep unless a separate migration requires change | No broad find-and-replace across identifiers or applied migrations |
| Historic audits and commits | Preserve original names | Add a contextual link, do not rewrite historical evidence |

Search visible strings in both apps, `public/sw.js`, web manifests, HTML titles, API-produced public messages, notification payloads, emails if any, favicon/OG assets, README, QR templates and screenshots used in documentation. Retain intentional historical references. Maintain one shared brand configuration consumed by both frontend builds; service-worker build constants must derive from the same source.

## Supplied identity and visual direction

Source: [unchanged supplied logo](assets/bal-setu-logo-source.jpeg), copied from `C:/Users/notic/Downloads/WhatsApp Image 2026-09-12 at 9.15.43 PM.jpeg` so another implementer can use it without access to the original machine. The supplied white-background image contains a purple/blue shield, two human figures, a teal accent and BAL SETU lettering. Preserve the human connection and bridge idea. The existing long technical tagline is not child-facing navigation.

Use the complete supplied logo in an About/footer setting; use a clean approved symbol and text wordmark in the header. Do not stretch, crop off figures or pretend the raster source is an editable vector. Archive an unchanged source copy when implementing. Any recolouring or redrawing requires visual comparison with the source; export 192/512 app icons, favicon, light/dark single-colour variants and an SVG only if genuinely vector. Logo alt text: “Bal Setu”; adjacent text makes the symbol decorative.

Signature: a modest curved bridge line joining two equal dots beside “You choose what to share.” It expresses connection without depicting an adult as automatically safe. Do not add injured children, sirens, flashing emergency graphics, police badges, reward points or celebratory confetti.

| Token | Value | Use |
| --- | --- | --- |
| `canvas` | `#FAFBFF` | Quiet page background |
| `surface` | `#FFFFFF` | Cards and inputs |
| `ink` | `#242238` | Main text |
| `primary` | `#603B91` | Main action, white text |
| `teal` | `#176B70` | Secondary action, white text |
| `lilac` | `#F0EAF8` | Help-choice surface, ink text |
| `sky` | `#EAF3FC` | Conversation/support surface, ink text |
| `peach` | `#FFF0E7` | Gentle attention surface, ink text |
| `danger` | `#A82D3C` | Errors and urgent labels, never decorative |
| `focus` | `#165EA8` | Visible focus ring with white separation |

Verify actual contrast after rendering: normal text 4.5:1, large text 3:1, essential control boundaries/icons 3:1. Never place white text on pastel fills. Avoid neon, glowing edges, glass effects and large gradients. Colour supports labels; it never conveys urgency/status alone.

Typography: self-host Noto Sans for body and controls; Noto Sans Devanagari/Gujarati for the corresponding languages. Use a modest rounded display face such as Nunito Sans for short Latin headings only, with matching script fallbacks. Include font licence files and subset responsibly. Body 18px, controls at least 16px, line height 1.55; headings 28–36px. Do not transform all text to uppercase. System-font fallback must remain usable before fonts load.

## Layout and components

Public content column: maximum 640px, 16px phone gutters, 24px desktop gutters. At 320px width all primary paths remain single-column. Staff desktop can use queue/detail columns; on phones use separate list/detail pages with an explicit Back button. No horizontal table required to read a case.

Persistent header: small Bal Setu mark, Language, **Leave this page**. Footer: immediate-help link, privacy explanation, staff sign-in. Do not make children choose between role dashboards. At most one visually dominant action per screen. Use text and icon together; target 48×48px, preferably 56px for primary actions. Leave 8px between targets. Sticky controls respect safe-area insets, zoom and the software keyboard.

Required shared components: `ChildShell`, `StepHeading`, `ChoiceButton`, `MessageComposer`, `VoiceRecorder`, `VoicePlayer`, `ListenButton`, `ServiceStatus`, `ReturnKeyCards`, `InlineError`, `ReceiptStatus`, `LocationChoice`, `AlertCard`, `ConfirmAction`. Build their presentation for this product; retain semantic HTML and established accessible primitives for focus/dialog behaviour.

Every step has a heading, one short explanation, primary action, Back where meaningful, and optional “Not sure”/“Skip” where allowed. Do not use disabled buttons without an adjacent reason. Focus new headings after navigation; preserve focus after inline errors. Use polite status announcements; never read private content aloud automatically. Escape closes ordinary dialogs, not the page; Quick Exit always remains available.

Respect reduced motion; state transitions under 180ms, no pulsing. Support keyboard, screen reader, switch control, 200% text resizing and 400% zoom/reflow. No drag/hold-only recording gesture. Dates use the chosen locale and explicit timezone for staff; child messages show familiar dates/times without raw ISO strings.

## Language and content rules

Initial reviewed languages: English, Hindi and Gujarati. Hide incomplete language choices rather than showing an untranslated journey. Store stable translation IDs, not English as IDs. Switching language preserves drafts, chosen location and return-word identity. It must never send data. Human-review safety copy and voice prompts; machine translation is not sufficient release approval. Future scripts/RTL should fit the component layout.

| Avoid | Use |
| --- | --- |
| Submit incident / raise ticket | Send to the support team |
| Token / session / capability | Your three words / your return card |
| Incognito / completely anonymous | You do not need to give your name |
| Tell us everything / provide evidence | Share as much or as little as you want |
| Victim / perpetrator in child form | You / the person you are worried about |
| Verified / safe / help is on its way | Received / a support worker opened your message |
| Geofenced broadcast | Get alerts for this area |
| No cases found | No alerts are published for this area right now |

Suggested home copy: “You can ask for help here.” Supporting line: “You can type, use your voice, or choose a button.” Never promise that every adult is safe, absolute confidentiality, instant replies or guaranteed rescue.

Privacy layer one: “The support team can read or hear what you send. They may need to involve other people to help protect you. You can ask how they will use it.” More detail identifies the actual organization, access, retention and escalation duties; obtain local legal/practitioner review. Do not require children to understand a long policy before asking for help.
