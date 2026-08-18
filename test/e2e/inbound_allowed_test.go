//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

// TestInboundAllowed: trunk-mock places an inbound call to device-1 from
// an allowlisted caller number. The backend should originate a second leg
// to device-1 and bridge the two, so the device-1 SIPp UAS should receive
// and answer a real call, and the backend should log the attempt as
// "connected".
func TestInboundAllowed(t *testing.T) {
	waitForAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceID := deviceForEndpoint(t, "device-1")
	const callerNumber = "15015550201"
	purgeCallsForNumber(t, deviceID, callerNumber)
	contactID := addContact(t, deviceID, callerNumber)
	defer deleteContact(t, contactID)
	defer purgeCallsForNumber(t, deviceID, callerNumber)

	uas := &SIPClient{ContainerName: sipDeviceContainer()}
	uasDone := uas.ListenAndAnswerAsync(ctx, 15*time.Second)
	time.Sleep(1 * time.Second)

	uac := &SIPClient{ContainerName: sipTrunkContainer()}
	if out, err := uac.PlaceCall(ctx, "device-1", asteriskHost, callerNumber, true); err != nil {
		t.Fatalf("place inbound call: %v\n%s", err, out)
	}

	call := waitForCallOutcome(t, deviceID, callerNumber)
	if call.Direction != "inbound" {
		t.Errorf("direction = %q, want inbound", call.Direction)
	}
	if call.Outcome != "connected" {
		t.Errorf("outcome = %q, want connected (reason=%v)", call.Outcome, call.Reason)
	}

	result := <-uasDone
	if result.Err != nil {
		t.Logf("device-1 UAS sipp exited with error (non-fatal, checking received flag): %v\n%s", result.Err, result.Out)
	}
	if !result.Received {
		t.Errorf("device-1 never received the bridged second leg; sipp output:\n%s", result.Out)
	}
}
