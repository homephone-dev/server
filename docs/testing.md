# Testing

## Unit tests

`internal/decision` and `internal/callctl` are fully unit tested with no
containers required:

```
go test ./internal/decision/... ./internal/callctl/...
```

`internal/decision.Evaluate` is a pure function (no I/O), so its tests cover
the decision matrix directly: allowed number in window, allowed number out of
window, disallowed number regardless of schedule, the deliberate
empty-allowlist-blocks-all rule, overlapping allow/block windows, and
multiple per-day windows.

`internal/callctl` is tested against a fake `ari.Client` (see
`internal/callctl/app_test.go`), asserting that a blocked call only ever
gets `Hangup` called on it (never `Answer`/bridge), and that an allowed call
originates a second leg, answers both legs, and bridges them.

## End-to-end tests

The e2e harness in `test/e2e/` places real SIP calls through a live
Asterisk instance and asserts outcomes via the backend REST API. It uses
[SIPp](https://github.com/SIPp/sipp) (Debian package `sip-tester`) instead
of `pjsua`: SIPp drives both call placement (UAC) and call answering (UAS)
purely via XML scenario files and CLI flags, with no interactive-CLI/stdin
scripting needed.

All four scenario tests are implemented and pass against a live stack:
`TestInboundAllowed`, `TestInboundBlocked`, `TestOutboundAllowed`,
`TestOutboundBlocked` (plus `TestSmoke_APIReachable`), in
`test/e2e/*_test.go` (package `e2e`, build-tagged `e2e`).

### How it works

- `test/e2e/Dockerfile.sipp` builds a single image (installs `sip-tester`)
  used for both `sip-device` and `sip-trunk-mock` in
  `docker-compose.e2e.yml`. Each container can act as UAC (placing a call)
  or UAS (answering one) depending on the scenario file SIPp is pointed at:
  - `test/e2e/sipp/uac.xml` -- places a call and blocks for a 200 OK, ACKs,
    holds briefly, then hangs up with BYE. Used when the call is expected
    to be **allowed**.
  - `test/e2e/sipp/uac_attempt.xml` -- places a call and does not wait for
    an answer. Used when the call is expected to be **blocked** (the
    backend hangs up the channel before answer, so the far end never gets
    a 200); the Go test doesn't require SIPp's own run to "succeed" for
    this scenario, it asserts the outcome via the REST API instead.
  - `test/e2e/sipp/uas.xml` -- listens for one INVITE, answers with real
    SDP, holds, and waits for BYE. Used on whichever side (device-1 or
    trunk-mock) is expected to receive the bridged second leg.
- `test/e2e/sipclient.go` wraps `docker exec <container> sipp ...` for
  both roles and reports back whether a call was placed/received.
- `test/e2e/main_test.go` seeds/cleans devices, contacts and call-log rows
  via the REST API and polls `GET /devices/{id}/calls` for a terminal
  outcome (`connected` or `blocked`).

### Asterisk config changes made to support this harness

- **`asterisk/etc/asterisk/extensions.conf`**: the dialplan pattern was
  `_X.` (digits only), which cannot match either an E.164-style dialed
  number starting with a non-digit-first pattern edge case, nor (more
  importantly) a literal endpoint id like `device-1` used as the dialed
  extension for inbound routing. Changed to `_.` (any one-or-more-char
  extension). This surfaced a second latent issue: Asterisk's implicit `h`
  (hangup) extension execution then also matched the catch-all pattern,
  re-entering `Stasis()` with a bogus `exten=h` on every hangup. Fixed by
  adding an explicit `exten => h,1,Hangup()` — literal extensions always
  take precedence over pattern matches in Asterisk, so this cleanly wins
  over `_.` for exactly the "h" case.
- **`asterisk/etc/asterisk/pjsip.conf`**: added `direct_media=no` to both
  endpoints. Without it, Asterisk renegotiates direct RTP between the two
  bridged legs immediately after answer (a second in-dialog INVITE), which
  broke the SIPp UAC scenario (unexpected message mid-call) and isn't
  needed here since Asterisk staying in the media path is fine for a
  call-screening PBX.
- **Auth simplification for `device-1` and `trunk-mock` (real design
  tradeoff, not just a testing hack — read this before changing pjsip.conf
  again):** these two endpoints no longer use SIP digest auth (`auth=`).
  Digest-auth scripting from SIPp's CLI is supported but adds real
  complexity for two dev-only, docker-compose-internal test fixtures whose
  passwords were already dev placeholders pre-e2e. Instead:
  - Each endpoint has a `type=identify` object matching by hostname
    (`match=sip-device` / `match=sip-trunk-mock`) — Asterisk resolves the
    Docker Compose service hostname to the container's IP at config-load
    time and matches inbound requests by source IP. Verified live: `pjsip
    show identifies` shows this resolving to a real `/32` IP.
  - Each endpoint's AOR has a **static contact**
    (`contact=sip:device-1@sip-device:5061`, etc.) instead of requiring
    REGISTER, since these are long-lived test fixtures, not dynamic phones.
  - **This only weakens SIP-level transport auth for these two named test
    endpoints on the docker-compose-internal network.** It has zero effect
    on the backend's allow/block decision logic
    (`internal/decision`, `internal/callctl`) — that logic only ever sees
    "which endpoint/exten dialed, which remote number", never anything
    resembling a credential, and is completely unchanged. A real
    deployment's actual outside-trunk and device endpoints should keep (or
    add) proper `auth=`; don't copy this pattern for anything reachable
    from outside the docker-compose network.

### Known SIPp quirks worth knowing if you touch the `.xml` scenarios

- SIPp 3.6.1 (the version in `debian:bookworm-slim`'s `sip-tester`
  package) crashes ("`Scenario command not implemented in display`", then
  aborts) on `<label>`/`next=` branching in at least this scenario shape.
  Scenarios here are deliberately kept linear (no `<label>`) to avoid it —
  hence two separate UAC scenarios (`uac.xml` / `uac_attempt.xml`) instead
  of one with a branch on the response code.
- SIPp also errors ("`<recv> before <X> sequence without a mandatory
  message`") if one or more consecutive `optional="true"` `<recv>`
  elements are immediately followed by a non-`<recv>` action (e.g.
  `<pause>`). A mandatory (non-optional) `<recv>` breaks the ambiguity.
- Custom scenario keywords are `-key <name> <value>` on the CLI, referenced
  as `[name]` in the XML — **not** `[$name]` (that's for `-set`/scenario
  variables). Getting this wrong doesn't error; it silently substitutes an
  empty string.
- `-timeout_error` takes no argument (it's a boolean flag). Passing a value
  after it (e.g. `-timeout_error 1`) gets consumed as SIPp's positional
  `remote_host` argument instead, silently breaking the target host.

### Running it

```
docker compose -f docker-compose.yml -f docker-compose.e2e.yml up -d --build
API_TOKEN=<your token> ARI_PASSWORD=<your ari password> POSTGRES_PASSWORD=<your pg password> \
  go test -tags e2e ./test/e2e/... -v
docker compose -f docker-compose.yml -f docker-compose.e2e.yml down -v
```

Run from the host (not inside a container) so `docker exec` can reach the
SIPp containers and `localhost:8080` reaches the published backend port.

**Note:** if you restart/recreate the `asterisk` container after the stack
is already up, the backend's ARI websocket connection to it drops
(`callctl: ARI event stream disconnected, reconnecting` in backend logs)
but the backend now reconnects on its own with exponential backoff once
Asterisk's ARI is back up — no manual `docker compose restart backend`
needed. Still prefer a single `down -v` + `up -d --build` cycle per test
run over restarting individual services mid-run, since in-flight call
state is reset on reconnect (see below).

Container names assumed by `test/e2e/main_test.go` follow docker compose's
default `<project>-<service>-<n>` naming with the project name derived
from the repo directory (`homephone-dev-server`). Override via the
`SIP_DEVICE_CONTAINER` / `SIP_TRUNK_CONTAINER` env vars if you run under a
different project name.

### Remaining known gaps

- The "blocked" scenarios don't assert on a specific SIP response code
  (see the SIPp `<label>` crash note above) — they assert the backend's
  logged outcome and, for two of the four tests, that the would-be
  answering side received zero calls. This is a slightly weaker signal
  than "the caller received exactly a 486" but still exercises the full
  real call path end-to-end.

## CI

- `.github/workflows/unit-tests.yml` runs `go test ./internal/...` on every
  push/PR, no Docker required.
- `.github/workflows/e2e-tests.yml` brings up the full compose stack
  (including the e2e overlay) and runs `go test -tags e2e ./test/e2e/...`.
  Given the gaps above, expect this workflow to need follow-up work before
  it's reliably green.
