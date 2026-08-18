//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

// TestInboundBlocked: trunk-mock places an inbound call to device-1 from a
// caller number that is NOT on device-1's allowlist. The backend should
// hang up the channel before answering and never originate a second leg --
// so device-1 should never see an INVITE, and the backend should log the
// attempt as "blocked".
func TestInboundBlocked(t *testing.T) {
	waitForAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceID := deviceForEndpoint(t, "device-1")
	const callerNumber = "15015550202" // deliberately not on the allowlist
	purgeCallsForNumber(t, deviceID, callerNumber)
	defer purgeCallsForNumber(t, deviceID, callerNumber)

	uas := &SIPClient{ContainerName: sipDeviceContainer()}
	uasDone := uas.ListenAndAnswerAsync(ctx, 6*time.Second)
	time.Sleep(1 * time.Second)

	uac := &SIPClient{ContainerName: sipTrunkContainer()}
	if out, err := uac.PlaceCall(ctx, "device-1", asteriskHost, callerNumber, false); err != nil {
		t.Logf("place inbound (blocked) call: %v\n%s", err, out) // best-effort, not fatal
	}

	call := waitForCallOutcome(t, deviceID, callerNumber)
	if call.Direction != "inbound" {
		t.Errorf("direction = %q, want inbound", call.Direction)
	}
	if call.Outcome != "blocked" {
		t.Errorf("outcome = %q, want blocked (reason=%v)", call.Outcome, call.Reason)
	}

	result := <-uasDone
	if result.Received {
		t.Errorf("device-1 received a call for a blocked attempt; sipp output:\n%s", result.Out)
	}
}
