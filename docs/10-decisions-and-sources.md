# Decisions and sources
Reviewed 12 September 2026. Recheck changing terms and operating requirements before launch.

- Working name: **Savera Alert**, with a dawn emblem. No victim or family association is claimed. AMBER is a design reference for controlled publication, not authorization to operate India's emergency network.
- Registered areas replace unpopulated geographic polygons for this release. Subscription is explicit; no live location tracking.
- Established protocols: Keycloak/go-oidc for staff identity and webpush-go for encrypted browser delivery. Do not rebuild password/OIDC/Web Push cryptography.
- Adult coding-assistant use is separate from child-facing runtime AI. Current runtime stays simulated/unavailable.

## Primary references
- [AMBER issuing criteria](https://amberalert.ojp.gov/about/guidelines-for-issuing-alerts): useful policy reference; US program, not Indian operating authority.
- [India MHA, 24 March 2026](https://www.mha.gov.in/MHA1/Par2017/pdfs/par2026-pdfs/LS24032026/5252.pdf): Mission Vatsalya/child-help context. No government API integration is configured here.
- [Keycloak JavaScript adapter](https://www.keycloak.org/securing-apps/javascript-adapter)
- [go-oidc](https://github.com/coreos/go-oidc)
- [webpush-go](https://github.com/SherClockHolmes/webpush-go)
- [WebKit Home Screen Web Push](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)
- [Firebase Cloud Messaging](https://firebase.google.com/products/cloud-messaging): transport context; current implementation uses direct Web Push.
- [Gemini API terms](https://ai.google.dev/gemini-api/terms): provider eligibility and data-use constraints.
- [POCSO Act](https://www.indiacode.nic.in/bitstream/123456789/2079/1/AA2012-32.pdf)
- [DPDP commencement notification](https://www.meity.gov.in/static/uploads/2025/11/c56ceae6c383460ca69577428d36828b.pdf): phased commencement; do not assume all duties began together.
- [CERT-In directions](https://cert-in.org.in/Directions70B.jsp)
- [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
