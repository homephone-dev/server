// Package callctl owns the per-channel Stasis application state machine. It
// is the only package that consumes both internal/ari and internal/decision,
// and it persists call outcomes via internal/store.
package callctl

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/zfand/homephone-dev-server/internal/ari"
	"github.com/zfand/homephone-dev-server/internal/decision"
	"github.com/zfand/homephone-dev-server/internal/store"
)

// DeviceLookup resolves the device configuration needed to evaluate a call
// attempt. Kept as an interface so callctl tests don't need a real database.
type DeviceLookup interface {
	DeviceBySIPEndpoint(ctx context.Context, endpoint string) (DeviceRecord, error)
	DeviceByID(ctx context.Context, id uuid.UUID) (DeviceRecord, error)
}

// CallLogger persists call attempts and their outcomes.
type CallLogger interface {
	CreateCallLog(ctx context.Context, c store.CallLog) (store.CallLog, error)
	UpdateCallLogByChannel(ctx context.Context, ariChannelID string, answeredAt, endedAt *time.Time, durationSeconds *int, outcome *string, reason *string) (store.CallLog, error)
}

type DeviceRecord struct {
	ID        uuid.UUID
	Timezone  *time.Location
	Allowlist []string
	Schedules []decision.ScheduleWindow
	// TrunkEndpoint is the ARI endpoint used to reach this device (e.g.
	// "PJSIP/device-1"). For outbound calls it's the mock-trunk endpoint.
	Endpoint string
}

const (
	MockTrunkEndpoint = "PJSIP/trunk-mock"
	bridgeType        = "mixing"
)

// App is the Stasis application controller: it owns the per-channel state
// machine driven by ARI events.
type App struct {
	ari     ari.Client
	devices DeviceLookup
	calls   CallLogger
	appName string
	logger  *slog.Logger

	mu        sync.Mutex
	pending   map[string]*callState // keyed by originating channel ID
	legToOrig map[string]string     // second-leg channel ID -> originating channel ID
	// endedChannels tracks channel IDs whose ChannelEnded event arrived
	// before we had a pending entry registered for them (e.g. the caller
	// hangs up while Originate is still in flight). Consulted right after
	// Originate returns so the orphaned second leg can be hung up instead
	// of tracked forever.
	endedChannels map[string]bool

	// reconnectInitialBackoff/reconnectMaxBackoff default to
	// initialReconnectBackoff/maxReconnectBackoff; tests shrink them so
	// reconnect scenarios run quickly.
	reconnectInitialBackoff time.Duration
	reconnectMaxBackoff     time.Duration
}

type callState struct {
	origChannelID   string
	secondChannelID string
	deviceID        uuid.UUID
	direction       decision.Direction
	remoteNumber    string
	bridgeID        string
	bridged         bool
	startedAt       time.Time
}

func New(client ari.Client, devices DeviceLookup, calls CallLogger, appName string, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		ari:           client,
		devices:       devices,
		calls:         calls,
		appName:       appName,
		logger:        logger,
		pending:       make(map[string]*callState),
		legToOrig:     make(map[string]string),
		endedChannels: make(map[string]bool),

		reconnectInitialBackoff: initialReconnectBackoff,
		reconnectMaxBackoff:     maxReconnectBackoff,
	}
}

const (
	initialReconnectBackoff = time.Second
	maxReconnectBackoff     = 30 * time.Second
)

// Run connects to the ARI event stream and processes events for the lifetime
// of ctx. If the connection is lost (dial failure or a broken stream), it
// reconnects with exponential backoff rather than giving up, so a transient
// Asterisk restart self-heals without restarting the backend process. Run
// only returns once ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	backoff := a.reconnectInitialBackoff
	connected := false

	for {
		events, errs, err := a.ari.Events(ctx, a.appName)
		if err != nil {
			a.logger.Error("callctl: failed to connect to ARI event stream, retrying", "error", err, "retry_in", backoff)
			if !a.sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = a.nextBackoff(backoff)
			continue
		}

		if connected {
			a.logger.Info("callctl: reconnected to ARI event stream")
			// Asterisk restarting invalidates every channel/bridge ID we had
			// tracked, and it won't replay the StasisStart events for calls
			// that were in flight when the connection dropped. There is no
			// cheap way to reconcile that state against Asterisk's post-
			// restart reality, so we drop it and start clean; any calls that
			// were mid-flight at the time of the drop are effectively lost
			// (Asterisk itself tore them down on restart anyway).
			a.resetState()
		} else {
			a.logger.Info("callctl: connected to ARI event stream")
		}
		connected = true
		backoff = a.reconnectInitialBackoff

		streamErr := a.runEventLoop(ctx, events, errs)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		a.logger.Error("callctl: ARI event stream disconnected, reconnecting", "error", streamErr, "retry_in", backoff)
		if !a.sleepOrDone(ctx, backoff) {
			return ctx.Err()
		}
		backoff = a.nextBackoff(backoff)
	}
}

// runEventLoop processes events from a single, already-established event
// stream until ctx is cancelled or the stream errors out.
func (a *App) runEventLoop(ctx context.Context, events <-chan ari.Event, errs <-chan error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-errs:
			if ok && err != nil {
				return err
			}
		case ev, ok := <-events:
			if !ok {
				return fmt.Errorf("callctl: event stream closed")
			}
			a.handleEvent(ctx, ev)
		}
	}
}

// sleepOrDone waits for d, returning false early (without waiting) if ctx is
// cancelled first.
func (a *App) sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (a *App) nextBackoff(cur time.Duration) time.Duration {
	next := cur * 2
	if next > a.reconnectMaxBackoff {
		next = a.reconnectMaxBackoff
	}
	return next
}

// resetState clears all in-memory call tracking. Called after a reconnect,
// since any channels tracked before the drop no longer correspond to
// anything real on the (likely restarted) Asterisk instance.
func (a *App) resetState() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending = make(map[string]*callState)
	a.legToOrig = make(map[string]string)
	a.endedChannels = make(map[string]bool)
}

func (a *App) handleEvent(ctx context.Context, ev ari.Event) {
	switch ev.Type {
	case ari.EventStasisStart:
		a.onStasisStart(ctx, ev)
	case ari.EventChannelStateChange:
		a.onChannelStateChange(ctx, ev)
	case ari.EventStasisEnd, ari.EventChannelDestroyed:
		a.onChannelEnded(ctx, ev)
	}
}

// direction + remote number are derived from the StasisStart args, populated
// by the single dialplan line: Stasis(homephone,${CHANNEL(endpoint)},${EXTEN}).
func parseStasisArgs(args []string) (endpoint, exten string) {
	if len(args) > 0 {
		endpoint = args[0]
	}
	if len(args) > 1 {
		exten = args[1]
	}
	return
}

// isSecondLeg reports whether this StasisStart is for a channel we
// originated ourselves as the far end of an allowed call.
func (a *App) isSecondLeg(channelID string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	orig, ok := a.legToOrig[channelID]
	return orig, ok
}
