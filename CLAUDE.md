# CLAUDE.md

Guidance for coding agents working in this repo. Keep this short; it's
invariants, not a tutorial.

## Core invariant: decision logic lives in Go, not Asterisk

All allow/block decision logic — matching a call's number against a
device's allowlist, checking the current time against a device's
schedule, deciding connect vs. block — lives in the Go backend
(`internal/decision`, `internal/callctl`). `internal/decision.Evaluate` is
the pure decision function; `internal/callctl` is the Stasis state machine
that acts on its result.

**Asterisk's dialplan and pjsip config must never contain allow/deny
logic.** Asterisk's only job is SIP registration, call setup, and media —
it hands every call to the Stasis app and does whatever the backend tells
it (answer/bridge or hang up). Don't add dialplan `GotoIf`s, extension
patterns, or pjsip ACLs that make allow/block decisions; that logic
belongs in `internal/decision`/`internal/callctl` so it stays testable,
auditable, and in one place.

## The empty-allowlist-blocks-all rule is deliberate

A device with no allowlist entries blocks all calls by default (fail
closed, not fail open). This is intentional, tested behavior in
`internal/decision` — do not "fix" it into fail-open without an explicit
product decision, since that would silently let every unrecognized caller
through.

## PII deletion must stay structural

`contacts.number` and `call_logs.remote_number` are PII. Deletion/erasure
(the endpoints documented in `docs/api.md`) must be enforced structurally:
real foreign-key `CASCADE` in the schema (`migrations/`) plus explicit
erasure endpoints, not app-level "best effort" cleanup code that can drift
out of sync with the schema and silently leave orphaned PII behind. If you
add a table that references `contacts` or `call_logs`, give it a `CASCADE`
(or an explicit, tested erasure path) — don't rely on remembering to purge
it manually elsewhere in the code.
