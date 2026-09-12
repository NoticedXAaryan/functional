---
name: implement-vertical-slice
description: Implement one Bal Suraksha workflow across UI, API, database and operations while preserving documented safety boundaries.
---

Read the current audit and the selected requirement. Identify the initiating user action, authorization, durable write, background processing and final visible outcome. Resolve missing connections before tests. Implement only the selected package with explicit error, empty and retry states. Add forward migrations when needed. Reuse established authentication and cryptographic libraries. Verify meaningful behavior after integration, then update the README and audit with evidence, limitations and remaining work. Never use real child data for development.
