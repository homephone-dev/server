// Package e2e drives the SIPp containers (sip-device / sip-trunk-mock, see
// docker-compose.e2e.yml) via `docker exec`, and drives the REST API to
// seed fixtures and assert outcomes. See docs/testing.md for the current
// state of this harness.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SIPClient drives one SIPp container, identified by its docker-compose
// container name (e.g. "homephone-dev-server-sip-device-1"), acting as
// either the calling party (UAC) or the receiving party (UAS) depending on
// which scenario a given e2e test needs.
type SIPClient struct {
	ContainerName string
}

const scenarioDir = "/sipp"

// PlaceCall runs SIPp as a UAC (caller) inside the container, placing one
// call to calledUser@asteriskHost using callerNumber as the SIP identity
// (From/Contact user). If expectAnswer is true, it uses the "expect
// answer" scenario (uac.xml), which blocks for a 200 OK, ACKs, holds the
// call briefly, and hangs up with BYE -- returning an error if no 200 ever
// arrives. If expectAnswer is false, it uses the "fire and forget"
// scenario (uac_attempt.xml): it just sends the INVITE and returns nil
// unconditionally, since blocked calls are expected to never be answered --
// the caller should assert the outcome via the backend REST API instead.
func (c *SIPClient) PlaceCall(ctx context.Context, calledUser, asteriskHost, callerNumber string, expectAnswer bool) (string, error) {
	scenario := "uac_attempt.xml"
	if expectAnswer {
		scenario = "uac.xml"
	}
	args := []string{
		"exec", "-i", c.ContainerName,
		"sipp", "-sf", scenarioDir + "/" + scenario,
		"-s", calledUser,
		asteriskHost + ":5060",
		"-m", "1",
		"-key", "callerid", callerNumber,
		"-timeout", "10s",
		"-timeout_error",
		"-nostdin",
	}
	out, err := runDockerExec(ctx, args...)
	if !expectAnswer {
		// Fire-and-forget: we only care that the INVITE was sent, not
		// whether SIPp considers the (never-answered) call "successful".
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("sipp uac (expect answer) failed: %w\n%s", err, out)
	}
	return out, nil
}

// UASResult is the outcome of a ListenAndAnswer run, meant to be sent back
// over a channel from a goroutine racing against PlaceCall.
type UASResult struct {
	Received bool
	Out      string
	Err      error
}

// ListenAndAnswer runs SIPp as a UAS (callee) inside the container,
// listening on port 5061 for one incoming INVITE. It answers with a real
// 200 OK/SDP, holds briefly, and waits for BYE. It blocks until SIPp exits
// (either after handling one call, or after timeout if none arrives) and
// reports whether a call was actually received, so blocked-call scenarios
// can assert zero calls reached the far end.
func (c *SIPClient) ListenAndAnswer(ctx context.Context, timeout time.Duration) (received bool, out string, err error) {
	args := []string{
		"exec", "-i", c.ContainerName,
		"sipp", "-sf", scenarioDir + "/uas.xml",
		"-i", "0.0.0.0", "-p", "5061",
		"-m", "1",
		"-timeout", fmt.Sprintf("%ds", int(timeout.Seconds())),
		"-timeout_error",
		"-nostdin",
	}
	out, runErr := runDockerExec(ctx, args...)
	// The reliable signal is simply: did SIPp ever see an incoming call at
	// all (vs. exiting on -m 1's timeout with zero calls received).
	received = extractCounter(out, "Incoming calls created") > 0
	return received, out, runErr
}

// ListenAndAnswerAsync runs ListenAndAnswer in a goroutine and returns a
// channel that receives exactly one UASResult when it's done. Callers
// should give the UAS a moment to bind before dialing in (SIPp needs to
// open its listening socket before the far end's INVITE can arrive).
func (c *SIPClient) ListenAndAnswerAsync(ctx context.Context, timeout time.Duration) <-chan UASResult {
	ch := make(chan UASResult, 1)
	go func() {
		received, out, err := c.ListenAndAnswer(ctx, timeout)
		ch <- UASResult{Received: received, Out: out, Err: err}
	}()
	return ch
}

func extractCounter(out, label string) int {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, label) {
			continue
		}
		fields := strings.Split(line, "|")
		last := strings.TrimSpace(fields[len(fields)-1])
		var n int
		if _, err := fmt.Sscanf(last, "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

// runDockerExec runs `docker <args...>` (typically `docker exec <container>
// sipp ...`) and captures combined output. Using `docker exec` directly
// against the container name (rather than `docker compose exec`, which
// needs the compose files/project context) keeps this independent of the
// working directory the test binary runs from.
func runDockerExec(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}
