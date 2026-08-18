package callctl

import (
	"context"
	"time"

	"github.com/zfand/homephone-dev-server/internal/ari"
	"github.com/zfand/homephone-dev-server/internal/decision"
	"github.com/zfand/homephone-dev-server/internal/store"
)

func (a *App) onStasisStart(ctx context.Context, ev ari.Event) {
	channelID := ev.Channel.ID

	if origID, ok := a.isSecondLeg(channelID); ok {
		a.onSecondLegStarted(ctx, origID, channelID)
		return
	}

	a.mu.Lock()
	_, alreadyTracked := a.pending[channelID]
	a.mu.Unlock()
	if alreadyTracked {
		a.logger.Warn("callctl: duplicate StasisStart for already-tracked channel, ignoring", "channel", channelID)
		return
	}

	endpoint, exten := parseStasisArgs(ev.Args)

	// Outbound: the originating endpoint is a known device dialing out.
	if dev, err := a.devices.DeviceBySIPEndpoint(ctx, endpoint); err == nil {
		a.evaluateAndAct(ctx, ev, dev, decision.Outbound, exten, MockTrunkEndpoint)
		return
	}

	// Inbound: the originating endpoint is the (mock) outside trunk; exten
	// identifies which device is being called.
	dev, err := a.devices.DeviceBySIPEndpoint(ctx, exten)
	if err != nil {
		a.logger.Error("callctl: unknown destination, hanging up", "channel", channelID, "exten", exten)
		_ = a.ari.Hangup(ctx, channelID, "normal")
		return
	}
	remoteNumber := ev.Channel.Caller.Number
	a.evaluateAndAct(ctx, ev, dev, decision.Inbound, remoteNumber, "PJSIP/"+dev.Endpoint)
}

func (a *App) evaluateAndAct(ctx context.Context, ev ari.Event, dev DeviceRecord, direction decision.Direction, remoteNumber, targetEndpoint string) {
	channelID := ev.Channel.ID
	now := time.Now()

	outcome := decision.Evaluate(
		decision.CallAttempt{Direction: direction, Number: remoteNumber},
		decision.DeviceConfig{Timezone: dev.Timezone, Allowlist: dev.Allowlist, Schedules: dev.Schedules},
		now,
	)

	logOutcome := "blocked"
	var reason *string
	if outcome == decision.Allow {
		// Not yet actually connected — this is a placeholder outcome until
		// the bridge is fully formed (see onSecondLegStarted). If the call
		// never connects (ring-no-answer, bridge setup failure handled
		// separately, caller hangs up first, etc.) this is the outcome
		// that sticks, which is exactly "missed".
		logOutcome = "missed"
	} else {
		r := "not in allowlist or outside allowed schedule"
		reason = &r
	}

	log, err := a.calls.CreateCallLog(ctx, store.CallLog{
		DeviceID:     dev.ID,
		Direction:    string(direction),
		RemoteNumber: remoteNumber,
		StartedAt:    now,
		Outcome:      logOutcome,
		Reason:       reason,
		AriChannelID: channelID,
	})
	if err != nil {
		a.logger.Error("callctl: failed to write call log", "error", err)
	}

	if outcome == decision.Block {
		_ = a.ari.Hangup(ctx, channelID, "normal")
		return
	}

	secondCh, err := a.ari.Originate(ctx, targetEndpoint, a.appName, "", remoteNumber)
	if err != nil {
		a.logger.Error("callctl: originate second leg failed", "error", err)
		_ = a.ari.Hangup(ctx, channelID, "normal")
		failedOutcome := "failed"
		if _, err := a.calls.UpdateCallLogByChannel(ctx, channelID, nil, ptrTime(time.Now()), nil, &failedOutcome, nil); err != nil {
			a.logger.Error("callctl: failed to update call log to failed", "error", err, "channel", channelID)
		}
		return
	}

	a.mu.Lock()
	alreadyEnded := a.endedChannels[channelID]
	delete(a.endedChannels, channelID)
	if !alreadyEnded {
		a.pending[channelID] = &callState{
			origChannelID:   channelID,
			secondChannelID: secondCh.ID,
			deviceID:        dev.ID,
			direction:       direction,
			remoteNumber:    remoteNumber,
			startedAt:       now,
		}
		a.legToOrig[secondCh.ID] = channelID
	}
	a.mu.Unlock()

	if alreadyEnded {
		// The original channel hung up while Originate was in flight, so
		// nothing ever registered this second leg as pending; hang it up
		// now instead of leaving it ringing/dialing forever untracked.
		_ = a.ari.Hangup(ctx, secondCh.ID, "normal")
		if _, err := a.calls.UpdateCallLogByChannel(ctx, channelID, nil, ptrTime(time.Now()), nil, nil, nil); err != nil {
			a.logger.Error("callctl: failed to update call log after early hangup", "error", err, "channel", channelID)
		}
	}

	_ = log
}

func (a *App) onSecondLegStarted(ctx context.Context, origID, secondID string) {
	a.mu.Lock()
	cs, ok := a.pending[origID]
	a.mu.Unlock()
	if !ok {
		return
	}

	if err := a.ari.Answer(ctx, origID); err != nil {
		a.logger.Error("callctl: answer orig failed", "error", err)
	}
	if err := a.ari.Answer(ctx, secondID); err != nil {
		a.logger.Error("callctl: answer second leg failed", "error", err)
	}

	bridge, err := a.ari.CreateBridge(ctx, bridgeType)
	if err != nil {
		a.logger.Error("callctl: create bridge failed", "error", err)
		a.failBridgeSetup(ctx, origID, secondID, "")
		return
	}
	if err := a.ari.AddChannelToBridge(ctx, bridge.ID, origID); err != nil {
		a.logger.Error("callctl: add orig channel to bridge failed", "error", err)
		a.failBridgeSetup(ctx, origID, secondID, bridge.ID)
		return
	}
	if err := a.ari.AddChannelToBridge(ctx, bridge.ID, secondID); err != nil {
		a.logger.Error("callctl: add second channel to bridge failed", "error", err)
		a.failBridgeSetup(ctx, origID, secondID, bridge.ID)
		return
	}

	a.mu.Lock()
	cs.bridgeID = bridge.ID
	cs.bridged = true
	a.mu.Unlock()

	now := time.Now()
	connected := "connected"
	if _, err := a.calls.UpdateCallLogByChannel(ctx, origID, &now, nil, nil, &connected, nil); err != nil {
		a.logger.Error("callctl: failed to update call log to connected", "error", err, "channel", origID)
	}
}

// failBridgeSetup rolls back a partially-formed bridge: both legs are hung up
// (best-effort — one or both may already be gone), the bridge itself is
// explicitly deleted if it was created, the in-memory tracking for this call
// is cleared, and the call log is marked "failed" so it doesn't linger as a
// bogus "connected" row with no working bridge behind it.
func (a *App) failBridgeSetup(ctx context.Context, origID, secondID, bridgeID string) {
	_ = a.ari.Hangup(ctx, origID, "normal")
	_ = a.ari.Hangup(ctx, secondID, "normal")
	if bridgeID != "" {
		if err := a.ari.DeleteBridge(ctx, bridgeID); err != nil {
			a.logger.Error("callctl: delete bridge after failed setup failed", "error", err, "bridge", bridgeID)
		}
	}

	a.mu.Lock()
	if cs, ok := a.pending[origID]; ok {
		delete(a.pending, origID)
		delete(a.legToOrig, cs.secondChannelID)
	}
	a.mu.Unlock()

	failed := "failed"
	if _, err := a.calls.UpdateCallLogByChannel(ctx, origID, nil, ptrTime(time.Now()), nil, &failed, nil); err != nil {
		a.logger.Error("callctl: failed to update call log to failed", "error", err, "channel", origID)
	}
}

func (a *App) onChannelStateChange(ctx context.Context, ev ari.Event) {
	// Reserved for future granular state tracking (e.g. ringing). The
	// authoritative transitions we act on are StasisStart / StasisEnd /
	// ChannelDestroyed.
	_ = ctx
	_ = ev
}

func (a *App) onChannelEnded(ctx context.Context, ev ari.Event) {
	channelID := ev.Channel.ID

	a.mu.Lock()
	origID := channelID
	if o, ok := a.legToOrig[channelID]; ok {
		origID = o
	}
	cs, ok := a.pending[origID]
	if ok {
		delete(a.pending, origID)
		delete(a.legToOrig, cs.secondChannelID)
	} else {
		// No pending entry yet — most likely the original channel hung up
		// before Originate returned. Remember it so evaluateAndAct can
		// avoid orphaning the second leg once Originate does return.
		a.endedChannels[channelID] = true
	}
	a.mu.Unlock()

	if !ok {
		return
	}

	now := time.Now()
	var duration *int
	if !cs.startedAt.IsZero() {
		d := int(now.Sub(cs.startedAt).Seconds())
		duration = &d
	}

	// Only hang up the leg that didn't already end on its own; the leg that
	// triggered this event is presumably already gone.
	other := cs.secondChannelID
	if channelID == cs.secondChannelID {
		other = cs.origChannelID
	}
	if other != channelID {
		_ = a.ari.Hangup(ctx, other, "normal")
	}

	if cs.bridged {
		if cs.bridgeID != "" {
			if err := a.ari.DeleteBridge(ctx, cs.bridgeID); err != nil {
				a.logger.Error("callctl: delete bridge on call end failed", "error", err, "bridge", cs.bridgeID)
			}
		}
		if _, err := a.calls.UpdateCallLogByChannel(ctx, cs.origChannelID, nil, &now, duration, nil, nil); err != nil {
			a.logger.Error("callctl: failed to update call log on end", "error", err, "channel", cs.origChannelID)
		}
		return
	}

	// The bridge never formed (still ringing/dialing) — the call was never
	// actually connected, so it's a missed call, not a connected one.
	missed := "missed"
	if _, err := a.calls.UpdateCallLogByChannel(ctx, cs.origChannelID, nil, &now, duration, &missed, nil); err != nil {
		a.logger.Error("callctl: failed to update call log to missed", "error", err, "channel", cs.origChannelID)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
