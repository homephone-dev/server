## What does this change do?

## Why?

## How was this tested?

- [ ] `go build ./...` / `go vet ./...`
- [ ] Unit tests (`go test ./internal/...`)
- [ ] E2E tests (`docs/testing.md`) — if applicable
- [ ] Manual test against `docker compose up`

## PII / logging / storage impact

Does this change touch `call_logs`, `contacts.number`, migrations, or the
PII-deletion endpoints (`docs/api.md`)? If so, briefly describe the impact
and confirm deletion paths stay structural (FK `CASCADE` / explicit erasure
endpoints), not app-level best-effort cleanup. If not applicable, write
"N/A".

## Checklist

- [ ] If this changes an invariant documented in `CLAUDE.md` (if present),
      I've updated it.
