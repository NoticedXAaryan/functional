# Bal Setu: child-centred experience specification

Version 1 · 13 September 2026 · Status: implementation specification, not a release report.

## Product intent

A child can ask for help without knowing what happened to them is called, writing a long account, giving their name, or creating an account. They can choose a few clear options, type, or record their voice. They remain in control of what is sent and what makes sound. A responsible human team receives the request and can continue the conversation. Nearby adults can separately choose to receive reviewed missing-child alerts.

This design uses the supplied **Bal Setu** logo and the proposed incident-based feature name **Nithari Alert**. The alert's everyday label is **Nearby missing-child alerts**. It is an independent service, not a government broadcast system.

The design is informed by published child-survivor guidance. It does not represent clinical experience, a completed child-participation study, or approval by a safeguarding organization. A practitioner and language reviewers must review the final experience before inviting children. Do not ask children to disclose abuse as a usability exercise.

## Read in this order

| Document | Implementation decisions |
| --- | --- |
| [01 Brand, language and components](01-brand-language-components.md) | Naming migration, supplied logo, visual tokens, accessible components and copy |
| [02 Child and community flows](02-child-community-flows.md) | Every public screen, choices, navigation, errors and outcomes |
| [03 Responder and operator flows](03-responder-operator-flows.md) | Staff access, assignment, conversations, escalation and alert review |
| [04 Return access and voice](04-return-access-and-voice.md) | Three-word experience, secure recovery, voice recording, playback and storage |
| [05 Nearby alerts](05-nearby-alerts.md) | Location permission, geographic matching, notification lifecycle and public safety |
| [06 Contracts and deployment](06-contracts-and-deployment.md) | Current-code gaps, proposed APIs/data and hosted integration |
| [07 Phased AI implementation](07-phased-ai-implementation.md) | Ordered work packages, acceptance criteria, release evidence and rollback |

These documents supersede conflicting **target experience** requirements in docs/01, docs/02 and docs/07. They do not retroactively change implementation history in docs/12–14. Existing data/privacy safeguards remain until explicitly replaced by the contracts here. A new name must not erase operational limitations.

## Current baseline and unresolved deployment evidence

The user reports that everything is deployed. This writing pass inspected local Vercel configurations, `render.yaml`, the beta Compose stack and docs/14. A separately named deployment strategy document and live URLs were not located in the repository. They have been requested. Treat deployment existence as user-reported and runtime settings as unverified; do not infer that the running deployment uses the local blueprint's values.

The local tree includes another contributor's staged staff-cookie/session changes and migration 015. Preserve them. The older handoff requires Keycloak, while current local authentication also supports server-side sessions. Reconcile against the actual deployed commit before implementation. No live environment, database or notification configuration was changed for this specification.

## Non-negotiable experience decisions

- **No mandatory narrative.** A selection such as “I need help” can start a private conversation. Operational routing still needs a real receiving team.
- **No fixed messaging hours.** Children can send whenever the service is available; responders can leave permitted in-app replies at any time. Optional safe-contact preferences govern external contact, not opening the app.
- **Three words are a memorable label, not sufficient security by themselves.** Pair them with a random device credential or recovery card. Do not replace a 256-bit capability with three guessable words.
- **Silence by default.** No automatic playback, recording, vibration, push permission prompt or disclosure on a lock screen.
- **Reporting does not publish.** A missing-child report remains private until independent authorized review permits a minimal public alert.
- **No fabricated completeness.** “Sent” means stored; “Read” requires an actual read event; push provider acceptance does not mean a person saw an alert.
- **No demo experiences in the deployed child journey.** Keep synthetic records and evaluation environments separated. Remove fake success states rather than merely hiding their labels.

## Research references

- [UNICEF/IRC child-survivor resource package](https://www.unicef.org/reports/caring-child-survivors-sexual-abuse-resource-package): foundation for survivor-centred choices and best interests; adapt with an Indian safeguarding practitioner.
- [UNICEF communication guidance](https://www.unicef.org/eca/stories/11-tips-communicating-your-teen): listening without forcing a conversation.
- [NHRC statement on missing children in Nithari](https://nhrc.nic.in/media/press-release/nhrc-asks-for-the-present-status-cbi-investigation-into-the-missing-persons-or-children-in-nithari-village-asks-to-specify-whether-all-missing-cases-have-been-entrusted-to-cbi): incident-name rationale only; not a claim of affiliation or current criminal findings.
- [Government child helpline information](https://www.spniwcd.wcd.gov.in/child-helpline): 1098 and integration with emergency response. Verify displayed resources for the rollout location.
- [MDN MediaRecorder](https://developer.mozilla.org/en-US/docs/Web/API/MediaRecorder), [local speech voices](https://developer.mozilla.org/en-US/docs/Web/API/SpeechSynthesisVoice/localService), [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API), [WebKit iOS web push](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/): capability-dependent implementation, not universal device guarantees.
