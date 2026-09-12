# Safety, privacy and threat model
This document identifies engineering and operating gates; it is not legal advice or a certification.

| Threat | Current mitigation | Remaining gate |
| --- | --- | --- |
| False public alert | Separate preparer/approver and explicit publication | Verified real issuer authority and documented criteria |
| Reporter exposed to implicated institution | Conflict flag exists | Enforced independent routing and access restrictions |
| Unsafe contact | Preference UI exists | Persist and enforce before response |
| Return-code guessing / denial of access | Current implementation is inadequate | Indexed high-entropy capabilities and isolated rate limits |
| Private data in public notifications | Generic payload and minimized public projection | Content review and incident procedure |
| Outbound endpoint abuse | HTTPS provider allowlist, timeout, no redirects | Ongoing provider compatibility and egress review |
| Stale/withdrawn sends | Dispatch-time state check and cancellation | Observe real-device lifecycle; accepted pushes cannot be recalled |
| Data retained indefinitely | No complete retention program | Defined schedules, legal holds, deletion, backups and restore |
| Misleading AI judgment | Demo-only guidance | Eligible provider and child-safety review before runtime inference |

Do not promise one-time server deletion unless retention and legal obligations are actually implemented. Quick exit does not erase browser history. Do not publish private allegations, survivor identity or unrestricted sighting maps. Staff notes must never appear in the public return view.

Before real data: have responsible operators review applicable POCSO reporting/publication duties, phased DPDP requirements, incident obligations and consent/notice design with qualified advice. See [sources](10-decisions-and-sources.md). No blanket NGO exception or government partnership is assumed.
