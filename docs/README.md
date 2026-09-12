# Bal Suraksha documentation

Start with the [application user guide](../README.md) to run the app and understand its screens. Read the [current audit](13-system-audit-and-completion-plan.md) for what works and what must be completed. The product remains a synthetic demonstration.

| Document | Purpose |
| --- | --- |
| [14 Current beta handoff](14-beta-handoff.md) | Latest engineering changes, real deployment setup and remaining operating requirements |
| [01 Product requirements](01-product-requirements.md) | Intended behavior and acceptance requirements |
| [02 Experience and content](02-experience-and-content.md) | Navigation, copy, states and accessibility |
| [03 Architecture](03-architecture.md) | System components and reliability boundaries |
| [04 Data and API](04-data-and-api.md) | Storage, state transitions and contracts |
| [05 Safety and privacy](05-safety-privacy-and-threat-model.md) | Threats and real-data gates |
| [06 Infrastructure and budget](06-infrastructure-and-budget.md) | Local setup, notifications and spending considerations |
| [07 Delivery plan](07-delivery-plan.md) | Larger phased work packages |
| [08 Testing and release](08-testing-and-release.md) | Behavioral acceptance specification |
| [09 Operations runbook](09-operations-runbook.md) | Invited push rehearsal and operating responsibilities |
| [10 Decisions and sources](10-decisions-and-sources.md) | Design decisions and primary references |
| [11 Reuse and authentication](11-reuse-and-authentication.md) | Keycloak and established components |
| [12 Historical status](12-implementation-status.md) | Earlier snapshot, superseded by 13 |
| [13 Current audit and completion plan](13-system-audit-and-completion-plan.md) | Present behavior, evidence and prioritized blockers |
| [AI handoff](ai/START-HERE.md) | Coding-assistant workflow and focused repository skills |
| [OpenAPI](../api/openapi.yaml) | API contract; historical portions still need reconciliation |
| [Fictional scenarios](fixtures/scenarios.json) | Synthetic demo/evaluation inputs |

Default to APP_MODE=demo, NOTIFICATION_MODE=DISABLED and AI_ENABLED=false. Test push requires explicitly invited consenting testers. Planned phases and checked task boxes are not production-readiness evidence. Current audit findings override historical claims that a capability is complete.

The [original blueprint](../product-design/bal-suraksha-system-blueprint.md) is historical context. Gemini may be used by an adult developer as a coding assistant; this does not establish eligibility for a child-facing runtime service.
