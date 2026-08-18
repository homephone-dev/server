//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

// TestOutboundAllowed: device-1 dials an allowlisted number. The backend
// should originate a second leg to the mock trunk (PJSIP/trunk-mock) and
// bridge the two, so the trunk-mock SIPp UAS should receive and answer a
// real call, and the backend should log the attempt as "connected".
func TestOutboundAllowed(t *testing.T) {
	waitForAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceID := deviceForEndpoint(t, "device-1")
	const dialedNumber = "15015550101"
	purgeCallsForNumber(t, deviceID, dialedNumber)
	contactID := addContact(t, deviceID, dialedNumber)
	defer deleteContact(t, contactID)
	defer purgeCallsForNumber(t, deviceID, dialedNumber)

	uas := &SIPClient{ContainerName: sipTrunkContainer()}
	uasDone := uas.ListenAndAnswerAsync(ctx, 15*time.Second)
	time.Sleep(1 * time.Second) // give the UAS time to bind before we dial

	uac := &SIPClient{ContainerName: sipDeviceContainer()}
	if out, err := uac.PlaceCall(ctx, dialedNumber, asteriskHost, "device-1", true); err != nil {
		t.Fatalf("place outbound call: %v\n%s", err, out)
	}

	call := waitForCallOutcome(t, deviceID, dialedNumber)
	if call.Direction != "outbound" {
		t.Errorf("direction = %q, want outbound", call.Direction)
	}
	if call.Outcome != "connected" {
		t.Errorf("outcome = %q, want connected (reason=%v)", call.Outcome, call.Reason)
	}

	result := <-uasDone
	if result.Err != nil {
		t.Logf("trunk-mock UAS sipp exited with error (non-fatal, checking received flag): %v\n%s", result.Err, result.Out)
	}
	if !result.Received {
		t.Errorf("trunk-mock never received the bridged second leg; sipp output:\n%s", result.Out)
	}
}
