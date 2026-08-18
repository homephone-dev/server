// Package decision implements the pure allow/block logic for call attempts.
// It has no I/O, no ARI dependency, and no database dependency, so it can be
// exhaustively unit tested without any external services.
package decision

import "time"

type Direction string

const (
	Inbound  Direction = "inbound"
	Outbound Direction = "outbound"
)

type Outcome string

const (
	Allow Outcome = "allow"
	Block Outcome = "block"
)

// CallAttempt describes a single call attempt being evaluated.
type CallAttempt struct {
	Direction Direction
	Number    string
}

// DeviceConfig is the per-device configuration needed to evaluate an attempt.
type DeviceConfig struct {
	Timezone  *time.Location
	Allowlist []string
	Schedules []ScheduleWindow
}

// ScheduleWindow is a single allow/block window. DayOfWeek nil means every day.
// Days follow time.Weekday numbering (0=Sunday .. 6=Saturday).
type ScheduleWindow struct {
	DayOfWeek *int
	Start     time.Duration // offset since midnight
	End       time.Duration // offset since midnight
	Mode      Outcome
}

// Evaluate decides whether a call attempt should be allowed, based solely on
// the given device configuration and the current time. It is a pure function:
// no I/O, deterministic given its inputs.
//
// Rule: an empty allowlist for a device means block-all — this is deliberate,
// not a bug. A number must be explicitly present in the allowlist to ever be
// allowed through, and the schedule then further restricts allowed windows.
func Evaluate(attempt CallAttempt, device DeviceConfig, now time.Time) Outcome {
	if !numberAllowed(attempt.Number, device.Allowlist) {
		return Block
	}
	if !inAllowedWindow(device.Schedules, device.Timezone, now) {
		return Block
	}
	return Allow
}

