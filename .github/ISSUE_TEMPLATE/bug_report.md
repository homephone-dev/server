---
name: Bug report
about: Something isn't working as expected
title: ""
labels: bug
---

## What happened

A clear description of the bug.

## Expected behavior

What you expected to happen instead.

## How are you running this?

- [ ] `docker compose up` (full stack)
- [ ] devcontainer (`.devcontainer/`)
- [ ] Bare metal / other (describe below)

## Steps to reproduce

1.
2.
3.

## Logs

Please include relevant output. If via docker compose:

```
docker compose logs backend
docker compose logs asterisk
```

If you can reproduce it against a specific call attempt, the row from
`GET /api/v1/devices/{id}/calls` (see `docs/api.md`) is very useful too.

## Environment

- OS/platform:
- Go version (`go version`), if running outside docker:
- Relevant env vars set (do NOT paste actual secret values, just which
  ones are set): e.g. `API_TOKEN`, `ARI_PASSWORD`, `POSTGRES_PASSWORD`

## Additional context

Anything else that might help (Asterisk config changes, network setup, etc).
