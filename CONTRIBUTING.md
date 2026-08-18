# Contributing

Thanks for taking a look at this project. It's an early (Milestone 1),
self-hosted SIP PBX backend — not production-hardened, and it handles real
personally-identifiable information (phone numbers, call history). Please
read the "Handling PII" section below before submitting changes that touch
logging or storage.

## Getting a dev environment running

The quickest path is the devcontainer, which shares the same
`docker-compose.yml` used for a real deployment (no separate/duplicated
Dockerfile setup):

1. Set `API_TOKEN` and `ARI_PASSWORD` in your shell (see the README
   quickstart for the full list of required env vars, and `.env.example`
   for a copy-pasteable template).
2. Open the repo in VS Code (or any devcontainer-compatible editor) and
   reopen in container — this targets the `backend` service's build stage
   (Go toolchain, no compiled binaries).
3. Alternatively, skip the devcontainer and just run
   `docker compose up --build` directly, per the README quickstart.

## Running tests

See `docs/testing.md` for the full picture. Short version:

- **Unit tests** (`internal/decision`, `internal/callctl`, no containers
  required):
  ```
  go test ./internal/...
  ```
- **Build/vet**, which CI also runs:
  ```
  go build ./...
  go vet ./...
  ```
- **End-to-end tests** place real SIP calls through a live Asterisk
  instance via the `docker-compose.e2e.yml` overlay and SIPp. They're
  build-tagged `e2e` and are not run as part of the normal unit-test loop —
  see `docs/testing.md` for the exact invocation and current known gaps.

## Code style

Idiomatic Go, standard library first. This codebase deliberately avoids
unnecessary abstraction — e.g. `internal/api` uses stdlib `net/http` with Go
1.22+ `ServeMux` pattern routing rather than a router dependency, and
`internal/ari` is a hand-rolled ARI client rather than a wrapped SDK. New
code should follow that bias: prefer the direct/boring solution over adding
a dependency or an indirection layer, unless there's a concrete reason not
to.

If a `CLAUDE.md` exists at the repo root, read it — it documents invariants
(such as where allow/block decision logic is allowed to live) that apply to
both human and AI-assisted contributions.

## Handling PII

`contacts.number` and `call_logs.remote_number` are personally identifiable
information, and `call_logs` is call-detail-record-style data. If your
change touches logging, storage, migrations, or the PII-deletion endpoints
documented in `docs/api.md`, please:

- Avoid adding new places numbers or call metadata get logged/persisted
  without a clear reason.
- Keep deletion/erasure paths structural (real foreign-key `CASCADE` and
  explicit erasure endpoints), not app-level "best effort" cleanup that can
  silently leave orphaned PII behind.
- Call out the PII implications explicitly in your PR description.

## Submitting a change

- Open a PR against `main`. Include what changed and why, and how you
  tested it (unit tests / e2e / manual `docker compose up`).
- Keep PRs focused — separate refactors from behavior changes where
  possible.
- CI runs `go build`, `go vet`, and unit tests on every PR
  (`.github/workflows/unit-tests.yml`); e2e tests run separately
  (`.github/workflows/e2e-tests.yml`).
- A maintainer will review for correctness, adherence to the invariants in
  `CLAUDE.md` (if present), and the PII handling notes above. Small,
  well-scoped PRs get reviewed fastest.

## Reporting security issues

Please do not open a public issue for a security vulnerability — see
`SECURITY.md`.
