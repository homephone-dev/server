# Home Phone PBX Backend

[![unit-tests](https://github.com/homephone-dev/server/actions/workflows/unit-tests.yml/badge.svg)](https://github.com/homephone-dev/server/actions/workflows/unit-tests.yml)
[![e2e-tests](https://github.com/homephone-dev/server/actions/workflows/e2e-tests.yml/badge.svg)](https://github.com/homephone-dev/server/actions/workflows/e2e-tests.yml)

A self-hosted SIP PBX for a home landline: Asterisk handles SIP
registration, call setup, and media (nothing more — no allow/block logic
lives in Asterisk or on any phone), and a Go backend owns 100% of the
allow/block decision: it checks every call attempt's number against a
per-device allowlist and the current time against a per-device schedule,
decides connect or block, and logs every attempt.

**What this is:** a backend + Asterisk config + REST API you self-host
(e.g. on a home server or small VPS) to control which numbers can call or
be called on a SIP phone/ATA, on a schedule, with every attempt logged.

**What this is not:** it does not include a PSTN trunk or SIP carrier —
you bring your own (e.g. a VoIP provider or SIP trunking service) and
point it at this stack. It also does not include an admin UI yet; today
you manage devices/contacts/schedules via the REST API directly (see
`docs/api.md`). A separate admin client app is planned as Milestone 2 and
is not part of this repo.

**Maturity:** this is Milestone 1 — functionally complete (unit tests and
a live-stack e2e SIP test suite pass), but early and not
production-hardened. Read `SECURITY.md` before exposing it beyond a
trusted local network: notably, auth is a single static bearer token and
the REST API itself does not terminate TLS (put a reverse proxy in front
of it for real deployments).

## Quickstart

```
export API_TOKEN=<a long random string>
export ARI_PASSWORD=<a long random string>
export POSTGRES_PASSWORD=<a long random string>
docker compose up --build
```

This brings up Postgres, runs migrations, starts Asterisk (SIP on
`5060/udp`, ARI on `8088`), and starts the backend (REST API on `8080`).

Once it's up:

```
curl -s -H "Authorization: Bearer $API_TOKEN" http://localhost:8080/api/v1/devices
```

See `docs/api.md` for the full REST API and `docs/testing.md` for how to
run unit and end-to-end tests. See `.env.example` for a copy-pasteable
template of every environment variable this stack uses.

## Required environment variables

| Var | Required | Purpose |
|---|---|---|
| `API_TOKEN` | yes, no default | Bearer token required on every REST API request. The backend refuses to start if unset. |
| `ARI_PASSWORD` | yes, no default | Password for the Asterisk ARI user the backend authenticates as. |
| `POSTGRES_PASSWORD` | yes, no default | Postgres password (used by both `postgres` and `backend`/`migrate` services). |
| `POSTGRES_USER` | no (`homephone`) | Postgres user. |
| `POSTGRES_DB` | no (`homephone`) | Postgres database name. |

The backend connects to Postgres with `sslmode=require` — see
`docker-compose.yml` / `internal/config`. Locally this is satisfied by a
self-signed dev certificate baked into the `postgres/Dockerfile` image;
that certificate is for encrypting the connection only, not for identity
verification, and is not meant for production use.

## Deployment recommendation: encrypt the data volume

`call_logs` is call-detail-record-style data and `contacts.number` /
`call_logs.remote_number` are personally identifiable information. The
Postgres data volume (`postgres-data` in `docker-compose.yml`) should live
on an encrypted disk — FileVault on macOS, LUKS on Linux, or your cloud
provider's encrypted-volume option — as a deployment-time recommendation on
top of the application-level protections (bearer-token auth, `sslmode=require`,
and the PII-deletion API documented in `docs/api.md`).

## Repository layout

- `cmd/backend` — HTTP API + ARI Stasis app controller entrypoint.
- `cmd/migrate` — applies `migrations/*.sql` via goose, then exits.
- `internal/decision` — pure allow/block logic, no I/O.
- `internal/ari` — hand-rolled Asterisk REST Interface client.
- `internal/callctl` — the Stasis application state machine (uses `ari` + `decision`).
- `internal/store` — Postgres persistence (pgx/pgxpool, hand-written SQL).
- `internal/api` — REST API (stdlib `net/http`, Go 1.22+ `ServeMux` routing).
- `internal/config` — environment-variable configuration loading.
- `internal/wiring` — adapts `store` to the interfaces `callctl` expects.
- `asterisk/` — Asterisk config + Dockerfile.
- `migrations/` — goose SQL migrations.
- `test/e2e/` — end-to-end test harness (see `docs/testing.md` for known gaps).
- `docs/api.md` — REST API reference, including the PII-deletion endpoints.
- `docs/testing.md` — how to run unit and e2e tests, and current e2e gaps.

## Development

A devcontainer (`.devcontainer/devcontainer.json`) targets the backend
service's build stage (Go toolchain, no compiled binaries) for day-to-day
backend development — build/run/unit-test. It shares the same
`docker-compose.yml` used for the real deployment, so there is no separate,
duplicated Dockerfile setup. The e2e SIP tooling runs via the
`docker-compose.e2e.yml` overlay instead, not inside the devcontainer.

See `CONTRIBUTING.md` for how to get a dev environment running, run
tests, and submit changes.

## Security

This project handles real PII (phone numbers, call history) and is
early-stage. See `SECURITY.md` for the current security posture (auth
model, TLS story, encryption-at-rest) and how to report a vulnerability.
