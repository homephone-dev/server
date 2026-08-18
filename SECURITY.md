# Security Policy

## Scope

This policy covers this repository only: the Go backend (`internal/`,
`cmd/`), the Asterisk configuration in `asterisk/`, and the Postgres
migrations in `migrations/`. It does not cover self-hosted deployment
choices you make (your reverse proxy, host OS, network exposure), though
we're happy to hear about gaps in the documentation around those.

## Reporting a vulnerability

Please do not open a public GitHub issue for a security vulnerability.

Preferred: use [GitHub's private security advisory feature](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)
for this repository ("Security" tab -> "Report a vulnerability").

Alternatively, email **security@\<your-domain\>** (placeholder — repo
maintainer: replace with a real contact address before publishing).

Please include what you found, how to reproduce it, and its potential
impact. We'll acknowledge receipt and aim to follow up with a fix or
mitigation timeline.

## Current security posture (read this before reporting "known" gaps)

This is Milestone 1 of a self-hosted personal-use project, not a hardened
multi-tenant product. The following are known, current design choices —
please still report them if you think the risk is underestimated, but they
are not surprises to us:

- **Auth is a single static bearer token.** Every REST API request is
  checked against one `API_TOKEN` value (`internal/api/auth.go`,
  constant-time comparison). There is no per-user or per-scope
  authentication/authorization — anyone with the token has full access to
  every device, contact, schedule, and call log.
- **No TLS on the REST API itself.** The backend's HTTP server
  (`cmd/backend/main.go`) listens with plain `net/http.Server.ListenAndServe`
  — there is no HTTPS/TLS termination in the application. If you expose the
  REST API (port `8080` by default) beyond `localhost`/a trusted private
  network, put a reverse proxy (e.g. Caddy, nginx, Traefik) in front of it
  to terminate TLS. Sending the bearer token over plain HTTP on an untrusted
  network exposes it.
- **No encryption at rest by default.** The Postgres data volume
  (`postgres-data`) is not encrypted by the application. See the README's
  "Deployment recommendation: encrypt the data volume" section — this
  relies on host-disk encryption (FileVault, LUKS, your cloud provider's
  encrypted-volume option) as an operator responsibility.
- **Postgres connections use `sslmode=require`, not verified TLS.** The
  backend and migration tooling connect to Postgres with `sslmode=require`,
  which encrypts the connection but does not verify server identity against
  a CA (see `docker-compose.yml`, `internal/config`). Locally this is
  satisfied by a self-signed dev certificate baked into `postgres/Dockerfile`;
  that certificate is not intended for production use.
- **The Asterisk endpoints in `asterisk/etc/asterisk/pjsip.conf` are
  test-only configuration** on an internal docker-compose network (used by
  the e2e suite), with dev-only placeholder credentials — see the comments
  in that file and `docs/testing.md`. They are not meant to be exposed
  publicly or reused as-is in a real deployment.

If you're deploying this for real use, treat it as: put it behind a
reverse proxy with real TLS, keep the API token secret and rotate it if
you suspect exposure, keep the Postgres volume on encrypted disk, and don't
expose SIP (`5060/udp`) or ARI (`8088`) beyond what your deployment
actually needs.
