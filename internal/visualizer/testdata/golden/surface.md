# Surfaces

## concierge

Callers: Advisor
Spec: `obidozoe/docs/openapi.yaml`

> every route takes the shared HTTP Basic pair plus a bidder identity header

> **Finding:** a read-only credential is refused only at persistence, after the write ran

| Operation | Kind | What it offers | Used by |
|---|---|---|---|
| `getUserList` | query | the seeker list, unpaged, newest first, filtered by a typed status | Advisor works a task |
| `getUserLogs` | query | the seeker audit trail | — |
| `deleteAccount` | command | delete a seeker's account | Seeker removes their concierge account (unobserved caller) |
