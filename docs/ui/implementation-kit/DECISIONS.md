# Fixed decisions: do not improvise

This file resolves choices left open in the first specification. Security/domain rules in parent docs/04–06 still apply.

1. **Platform name:** Bal Setu. **Proposed alert name:** Nithari Alert. **Descriptive label:** Nearby missing-child alerts. The incident name remains subject to the operator/affected-community review described in doc/01. Do not invent a child victim's name or endorsement. If declined, change only the configured feature name to Setu Alert.
2. **Fonts for the first integration:** system-ui, -apple-system, Segoe UI, sans-serif. This supersedes the optional custom-font suggestion until reviewed font files and licences are bundled. It avoids requiring a weak implementer to source fonts. No Google Fonts requests from private pages.
3. **Colours and dimensions:** use tokens.css without ad hoc accent colours. White cards; purple main action; teal secondary emphasis; sky/lilac/peach supporting surfaces. No dark/neon/glass themes.
4. **Header logo:** use the app-icon master at 40px with separate live text “Bal Setu”. Do not render the full JPEG tagline in a tiny header. Original full logo may be used in About/footer with room to read it. Keep logo text accessible as normal text.
5. **Illustration:** optional homepage decoration, width up to 320px, `alt=""`. On short phones omit it if it pushes Ask for help below the first screen. It is never a button, a loading state or a safety guarantee.
6. **Help CTA:** Ask for help. **Send CTA:** Send to the support team. **Exit:** Leave this page. **Return:** My messages. Keep wording consistent.
7. **Home:** four actions. Staff sign-in only in footer. No statistics, supporter counters, testimonials, autoplay, carousel or rotating inspirational slogans.
8. **Public flow:** private by default; no mandatory narrative, name, phone, email, Aadhaar, exact date of birth or parent account. Sending requires a valid configured receiving team.
9. **Replies:** ask explicitly. New yes choice permits in-app replies any time; no private push. Existing no-contact/time-limited permission is not widened by migration. Opening the app is never gated by a schedule.
10. **Return words:** three word slots are labels combined with a secure device proof or return card. No global phrase search, no public uniqueness check, no three-word-only cross-device recovery. Word selection is an opt-in post-receipt subflow, not a condition for requesting help.
11. **Return expiry:** initially retain existing 30-day capability lifetime and 15-minute active access; present expiry in readable local language. Never revoke a valid legacy code merely because branding changes.
12. **Voice:** initial maximum 120s, 8 MiB, three clips per message; no transcription, voice biometrics or cloud speech service by default. A child can send choices/text if microphone support is absent. Staff voice replies include a typed equivalent.
13. **Push:** public alerts only. Permission after explicit tap, selected area registration, no claim of contacting every phone nearby. The user's position is used only after consent and area confirmation.
14. **Map:** an optional aid. Initial area list comes from actual reviewed operator records. No map SDK or paid geocoder needed for a manual-area beta. Do not draw fictional jurisdictions.
15. **Empty states:** text and a small icon. Do not add crying children, sad mascots or celebration to disclosure/receipt screens.
16. **Drafts:** memory only by default. No sensitive localStorage, service-worker caching or analytics replay. Exit destroys unsent memory and stops media before navigation.
17. **Build capability rules:** API-provided capability false means explain unavailability and retain alternatives. Hidden mock behaviour is never acceptable. A network timeout is not successful submission.
18. **Live API origins:** resolve deployment strategy first. Never ship localhost defaults in a production build or remove CSRF protections to make cross-origin cookies work.
19. **Languages:** English design copy is supplied; enable Hindi/Gujarati only after complete reviewed catalogs and script/device checks. Do not claim that a flag or translated homepage localizes the whole workflow.
20. **QR graphics:** generate a real QR only from a verified active entry point URL. No stock QR or fake location asset is provided. A print-ready QR poster must be created at that stage, with the actual operator and destination.

## Word-selection subflow details

C07 branches first. Device selected → explain persistent access → choose slot 1, 2, 3 → confirm all three with text/picture labels → persist after explicit confirmation → conversation. Card selected → explain file possession risk → choose three labels → explicit Generate card → show Download/Print options → conversation. Not now → conversation with no persistence.

Do not embed a tiny demonstration word list as a production credential dictionary. Production word IDs and translations need a reviewed stable vocabulary. While unavailable, keep the existing secure return-code method and clearly state that the improved words flow is not enabled. The UI must not present static example words as a user's credential.
