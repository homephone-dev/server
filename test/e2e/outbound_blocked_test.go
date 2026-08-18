//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

// TestOutboundBlocked: device-1 dials a number that is NOT on its
// allowlist. The backend should hang up the channel before answering and
// never originate a second leg -- so trunk-mock should never see an
// INVITE, and the backend should log the attempt as "blocked".
func TestOutboundBlocked(t *testing.T) {
	waitForAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceID := deviceForEndpoint(t, "device-1")
	const dialedNumber = "15015550102" // deliberately not on the allowlist
	purgeCallsForNumber(t, deviceID, dialedNumber)
	defer purgeCallsForNumber(t, deviceID, dialedNumber)

	// Confirm trunk-mock never receives a call: listen with a short bound.
	uas := &SIPClient{ContainerName: sipTrunkContainer()}
	uasDone := uas.ListenAndAnswerAsync(ctx, 6*time.Second)
	time.Sleep(1 * time.Second)

	uac := &SIPClient{ContainerName: sipDeviceContainer()}
	if out, err := uac.PlaceCall(ctx, dialedNumber, asteriskHost, "device-1", false); err != nil {
		t.Logf("place outbound (blocked) call: %v\n%s", err, out) // best-effort, not fatal
	}

	call := waitForCallOutcome(t, deviceID, dialedNumber)
	if call.Direction != "outbound" {
		t.Errorf("direction = %q, want outbound", call.Direction)
	}
	if call.Outcome != "blocked" {
		t.Errorf("outcome = %q, want blocked (reason=%v)", call.Outcome, call.Reason)
	}

	result := <-uasDone
	if result.Received {
		t.Errorf("trunk-mock received a call for a blocked attempt; sipp output:\n%s", result.Out)
	}
}
