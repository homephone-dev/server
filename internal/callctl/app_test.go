package callctl

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zfand/homephone-dev-server/internal/ari"
	"github.com/zfand/homephone-dev-server/internal/store"
)

type fakeARI struct {
	answered      []string
	hungup        []string
	bridged       [][2]string
	deletedBridge []string
	originate     func(endpoint, appName, appArgs, callerID string) (ari.Channel, error)
	createBridge  func() (ari.Bridge, error)
	addToBridge   func(bridgeID, channelID string) error
	bridgeID      string
}

func (f *fakeARI) Answer(ctx context.Context, channelID string) error {
	f.answered = append(f.answered, channelID)
	return nil
}
func (f *fakeARI) Hangup(ctx context.Context, channelID string, reason string) error {
	f.hungup = append(f.hungup, channelID)
	return nil
}
func (f *fakeARI) Originate(ctx context.Context, endpoint, appName, appArgs, callerID string) (ari.Channel, error) {
	if f.originate != nil {
		return f.originate(endpoint, appName, appArgs, callerID)
	}
	return ari.Channel{ID: "second-leg"}, nil
}
func (f *fakeARI) CreateBridge(ctx context.Context, bridgeType string) (ari.Bridge, error) {
	if f.createBridge != nil {
		return f.createBridge()
	}
	f.bridgeID = "bridge-1"
	return ari.Bridge{ID: f.bridgeID}, nil
}
func (f *fakeARI) AddChannelToBridge(ctx context.Context, bridgeID, channelID string) error {
	if f.addToBridge != nil {
		if err := f.addToBridge(bridgeID, channelID); err != nil {
			return err
		}
	}
	f.bridged = append(f.bridged, [2]string{bridgeID, channelID})
	return nil
}
func (f *fakeARI) DeleteBridge(ctx context.Context, bridgeID string) error {
	f.deletedBridge = append(f.deletedBridge, bridgeID)
	return nil
}
func (f *fakeARI) Events(ctx context.Context, appName string) (<-chan ari.Event, <-chan error, error) {
	return nil, nil, nil
}

type fakeDevices struct {
	byEndpoint map[string]DeviceRecord
}

func (f *fakeDevices) DeviceBySIPEndpoint(ctx context.Context, endpoint string) (DeviceRecord, error) {
	d, ok := f.byEndpoint[endpoint]
	if !ok {
		return DeviceRecord{}, errNotFound
	}
	return d, nil
}
func (f *fakeDevices) DeviceByID(ctx context.Context, id uuid.UUID) (DeviceRecord, error) {
	for _, d := range f.byEndpoint {
		if d.ID == id {
			return d, nil
		}
	}
	return DeviceRecord{}, errNotFound
}

var errNotFound = &notFoundErr{}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "not found" }

type callUpdate struct {
	channelID string
	outcome   *string
	endedAt   *time.Time
}

type fakeCalls struct {
	created []store.CallLog
	updated int
	updates []callUpdate
}

func (f *fakeCalls) CreateCallLog(ctx context.Context, c store.CallLog) (store.CallLog, error) {
	c.ID = uuid.New()
	f.created = append(f.created, c)
	return c, nil
}
func (f *fakeCalls) UpdateCallLogByChannel(ctx context.Context, ariChannelID string, answeredAt, endedAt *time.Time, durationSeconds *int, outcome *string, reason *string) (store.CallLog, error) {
	f.updated++
	f.updates = append(f.updates, callUpdate{channelID: ariChannelID, outcome: outcome, endedAt: endedAt})
	return store.CallLog{}, nil
}

func (f *fakeCalls) lastOutcome(channelID string) string {
	for i := len(f.updates) - 1; i >= 0; i-- {
		if f.updates[i].channelID == channelID && f.updates[i].outcome != nil {
			return *f.updates[i].outcome
		}
	}
	return ""
}

// reconnectFakeARI simulates a sequence of ARI event-stream connections: a
// dial failure, then a stream that breaks after emitting one event, then a
// stable stream that stays open until ctx is cancelled.
type reconnectFakeARI struct {
	fakeARI

	mu    sync.Mutex
	calls int
}

func (f *reconnectFakeARI) Events(ctx context.Context, appName string) (<-chan ari.Event, <-chan error, error) {
	f.mu.Lock()
	f.calls++
	n := f.calls
	f.mu.Unlock()

	switch n {
	case 1:
		return nil, nil, fmt.Errorf("dial refused")
	case 2:
		events := make(chan ari.Event, 1)
		errs := make(chan error, 1)
		events <- ari.Event{Type: ari.EventChannelStateChange}
		errs <- fmt.Errorf("EOF")
		return events, errs, nil
	default:
		events := make(chan ari.Event)
		errs := make(chan error)
		go func() {
			<-ctx.Done()
		}()
		return events, errs, nil
	}
}

func (f *reconnectFakeARI) connectCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestRun_ReconnectsAfterDialFailureAndStreamDrop(t *testing.T) {
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{}}
	calls := &fakeCalls{}
	fa := &reconnectFakeARI{}
	app := New(fa, devices, calls, "homephone", nil)
	app.reconnectInitialBackoff = time.Millisecond
	app.reconnectMaxBackoff = 5 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for fa.connectCalls() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if fa.connectCalls() < 3 {
		t.Fatalf("expected at least 3 connection attempts (initial failure, drop, stable), got %d", fa.connectCalls())
	}

	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected Run to return ctx error on shutdown, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}
}

func TestRun_ResetsStateOnReconnect(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+19998887777"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	app := New(fa, devices, calls, "homephone", nil)

	// Simulate in-flight call state left over from before a drop.
	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)

	app.mu.Lock()
	pendingBefore := len(app.pending)
	app.mu.Unlock()
	if pendingBefore == 0 {
		t.Fatalf("expected pending call state before reset")
	}

	app.resetState()

	app.mu.Lock()
	defer app.mu.Unlock()
	if len(app.pending) != 0 || len(app.legToOrig) != 0 {
		t.Fatalf("expected state cleared after resetState, got pending=%v legToOrig=%v", app.pending, app.legToOrig)
	}
}

func TestOnStasisStart_BlockedInbound_HangsUpOnly(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+15550001111"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	app := New(fa, devices, calls, "homephone", nil)

	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)

	if len(fa.hungup) != 1 || fa.hungup[0] != "orig-1" {
		t.Fatalf("expected exactly one hangup of orig-1, got %v", fa.hungup)
	}
	if len(fa.answered) != 0 {
		t.Fatalf("blocked call must never be answered, got %v", fa.answered)
	}
	if len(fa.bridged) != 0 {
		t.Fatalf("blocked call must never be bridged, got %v", fa.bridged)
	}
	if len(calls.created) != 1 || calls.created[0].Outcome != "blocked" {
		t.Fatalf("expected one blocked call log, got %+v", calls.created)
	}
}

func TestOnStasisStart_AllowedInbound_OriginatesAndBridges(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+19998887777"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	app := New(fa, devices, calls, "homephone", nil)

	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)

	if len(fa.hungup) != 0 {
		t.Fatalf("allowed call must not be hung up during origination, got %v", fa.hungup)
	}
	if len(calls.created) != 1 || calls.created[0].Outcome != "missed" {
		t.Fatalf("expected one placeholder 'missed' call log before the bridge forms, got %+v", calls.created)
	}

	// second leg StasisStart arrives
	secondEv := ari.Event{Type: ari.EventStasisStart, Channel: ari.Channel{ID: "second-leg"}}
	app.handleEvent(context.Background(), secondEv)

	if len(fa.answered) != 2 {
		t.Fatalf("expected both legs answered, got %v", fa.answered)
	}
	if len(fa.bridged) != 2 {
		t.Fatalf("expected both legs added to bridge, got %v", fa.bridged)
	}
	if got := calls.lastOutcome("orig-1"); got != "connected" {
		t.Fatalf("expected outcome updated to 'connected' once bridge forms, got %q", got)
	}
}

func TestOnSecondLegStarted_BridgeCreateFails_RollsBackAndMarksFailed(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+19998887777"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	fa.createBridge = func() (ari.Bridge, error) {
		return ari.Bridge{}, fmt.Errorf("asterisk unavailable")
	}
	app := New(fa, devices, calls, "homephone", nil)

	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)

	secondEv := ari.Event{Type: ari.EventStasisStart, Channel: ari.Channel{ID: "second-leg"}}
	app.handleEvent(context.Background(), secondEv)

	if got := calls.lastOutcome("orig-1"); got != "failed" {
		t.Fatalf("expected outcome 'failed' after bridge-create failure, got %q", got)
	}
	if len(fa.hungup) != 2 {
		t.Fatalf("expected both legs hung up after bridge-create failure, got %v", fa.hungup)
	}

	app.mu.Lock()
	_, stillPending := app.pending["orig-1"]
	app.mu.Unlock()
	if stillPending {
		t.Fatal("expected pending state cleared after bridge-create failure")
	}
}

func TestOnSecondLegStarted_AddChannelFails_DeletesBridgeAndMarksFailed(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+19998887777"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	fa.addToBridge = func(bridgeID, channelID string) error {
		return fmt.Errorf("channel already gone")
	}
	app := New(fa, devices, calls, "homephone", nil)

	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)

	secondEv := ari.Event{Type: ari.EventStasisStart, Channel: ari.Channel{ID: "second-leg"}}
	app.handleEvent(context.Background(), secondEv)

	if got := calls.lastOutcome("orig-1"); got != "failed" {
		t.Fatalf("expected outcome 'failed' after add-to-bridge failure, got %q", got)
	}
	if len(fa.deletedBridge) != 1 || fa.deletedBridge[0] != "bridge-1" {
		t.Fatalf("expected the created bridge to be explicitly deleted, got %v", fa.deletedBridge)
	}
}

func TestOnChannelEnded_RingNoAnswer_MarksMissed(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+19998887777"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	app := New(fa, devices, calls, "homephone", nil)

	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)

	// Caller hangs up before the second leg ever answers/bridges.
	endEv := ari.Event{Type: ari.EventStasisEnd, Channel: ari.Channel{ID: "orig-1"}}
	app.handleEvent(context.Background(), endEv)

	if got := calls.lastOutcome("orig-1"); got != "missed" {
		t.Fatalf("expected outcome 'missed' on ring-no-answer hangup, got %q", got)
	}
}

func TestOnStasisStart_DuplicateForTrackedChannel_Ignored(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+19998887777"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	app := New(fa, devices, calls, "homephone", nil)

	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)
	app.handleEvent(context.Background(), ev) // redelivered StasisStart

	if len(calls.created) != 1 {
		t.Fatalf("expected exactly one call log despite duplicate StasisStart, got %d", len(calls.created))
	}
}

func TestEvaluateAndAct_OrigHangsUpBeforeOriginateReturns_HangsUpSecondLeg(t *testing.T) {
	deviceID := uuid.New()
	devices := &fakeDevices{byEndpoint: map[string]DeviceRecord{
		"device-1": {ID: deviceID, Timezone: time.UTC, Allowlist: []string{"+19998887777"}, Endpoint: "device-1"},
	}}
	calls := &fakeCalls{}
	fa := &fakeARI{}
	app := New(fa, devices, calls, "homephone", nil)

	fa.originate = func(endpoint, appName, appArgs, callerID string) (ari.Channel, error) {
		// Simulate the original channel hanging up while Originate is in
		// flight, before the pending entry is registered.
		endEv := ari.Event{Type: ari.EventStasisEnd, Channel: ari.Channel{ID: "orig-1"}}
		app.handleEvent(context.Background(), endEv)
		return ari.Channel{ID: "second-leg"}, nil
	}

	ev := ari.Event{
		Type:    ari.EventStasisStart,
		Channel: ari.Channel{ID: "orig-1", Caller: ari.CallerID{Number: "+19998887777"}},
		Args:    []string{"trunk-mock", "device-1"},
	}
	app.handleEvent(context.Background(), ev)

	found := false
	for _, h := range fa.hungup {
		if h == "second-leg" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected orphaned second leg to be hung up, got hungup=%v", fa.hungup)
	}

	app.mu.Lock()
	_, pending := app.pending["orig-1"]
	app.mu.Unlock()
	if pending {
		t.Fatal("expected no pending entry registered for a channel that already ended")
	}
}
