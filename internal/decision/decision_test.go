package decision

import "testing"
import "time"

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return loc
}

func TestEvaluate_AllowedNumberInWindow(t *testing.T) {
	loc := mustLoc(t, "UTC")
	day := 3 // Wednesday
	device := DeviceConfig{
		Timezone:  loc,
		Allowlist: []string{"+15551234567"},
		Schedules: []ScheduleWindow{
			{DayOfWeek: &day, Start: 9 * time.Hour, End: 17 * time.Hour, Mode: Allow},
		},
	}
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, loc) // Wed
	got := Evaluate(CallAttempt{Direction: Inbound, Number: "+15551234567"}, device, now)
	if got != Allow {
		t.Fatalf("expected Allow, got %v", got)
	}
}

func TestEvaluate_AllowedNumberOutOfWindow(t *testing.T) {
	loc := mustLoc(t, "UTC")
	day := 3
	device := DeviceConfig{
		Timezone:  loc,
		Allowlist: []string{"+15551234567"},
		Schedules: []ScheduleWindow{
			{DayOfWeek: &day, Start: 9 * time.Hour, End: 17 * time.Hour, Mode: Allow},
		},
	}
	now := time.Date(2026, 8, 5, 20, 0, 0, 0, loc)
	got := Evaluate(CallAttempt{Direction: Inbound, Number: "+15551234567"}, device, now)
	if got != Block {
		t.Fatalf("expected Block, got %v", got)
	}
}

func TestEvaluate_DisallowedNumberRegardlessOfSchedule(t *testing.T) {
	loc := mustLoc(t, "UTC")
	device := DeviceConfig{
		Timezone:  loc,
		Allowlist: []string{"+15551234567"},
		Schedules: nil,
	}
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, loc)
	got := Evaluate(CallAttempt{Direction: Inbound, Number: "+15559999999"}, device, now)
	if got != Block {
		t.Fatalf("expected Block, got %v", got)
	}
}

func TestEvaluate_EmptyAllowlistBlocksAll(t *testing.T) {
	loc := mustLoc(t, "UTC")
	device := DeviceConfig{Timezone: loc, Allowlist: nil, Schedules: nil}
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, loc)
	got := Evaluate(CallAttempt{Direction: Outbound, Number: "+15551234567"}, device, now)
	if got != Block {
		t.Fatalf("empty allowlist must block-all, got %v", got)
	}
}

func TestEvaluate_OverlappingWindowsBlockTakesPrecedence(t *testing.T) {
	loc := mustLoc(t, "UTC")
	day := 3
	device := DeviceConfig{
		Timezone:  loc,
		Allowlist: []string{"+15551234567"},
		Schedules: []ScheduleWindow{
			{DayOfWeek: &day, Start: 8 * time.Hour, End: 20 * time.Hour, Mode: Allow},
			{DayOfWeek: &day, Start: 12 * time.Hour, End: 13 * time.Hour, Mode: Block},
		},
	}
	now := time.Date(2026, 8, 5, 12, 30, 0, 0, loc)
	got := Evaluate(CallAttempt{Direction: Inbound, Number: "+15551234567"}, device, now)
	if got != Block {
		t.Fatalf("expected Block due to overlapping block window, got %v", got)
	}
}

func TestEvaluate_MultipleAllowWindowsCoverDifferentDays(t *testing.T) {
	loc := mustLoc(t, "UTC")
	wed := 3
	sat := 6
	device := DeviceConfig{
		Timezone:  loc,
		Allowlist: []string{"+15551234567"},
		Schedules: []ScheduleWindow{
			{DayOfWeek: &wed, Start: 9 * time.Hour, End: 17 * time.Hour, Mode: Allow},
			{DayOfWeek: &sat, Start: 10 * time.Hour, End: 14 * time.Hour, Mode: Allow},
		},
	}
	satNow := time.Date(2026, 8, 8, 11, 0, 0, 0, loc)
	got := Evaluate(CallAttempt{Direction: Inbound, Number: "+15551234567"}, device, satNow)
	if got != Allow {
		t.Fatalf("expected Allow on saturday window, got %v", got)
	}
	sunNow := time.Date(2026, 8, 9, 11, 0, 0, 0, loc)
	got = Evaluate(CallAttempt{Direction: Inbound, Number: "+15551234567"}, device, sunNow)
	if got != Block {
		t.Fatalf("expected Block on sunday (no window), got %v", got)
	}
}

func TestEvaluate_NoSchedulesMeansAlwaysAllowed(t *testing.T) {
	loc := mustLoc(t, "UTC")
	device := DeviceConfig{Timezone: loc, Allowlist: []string{"+15551234567"}, Schedules: nil}
	now := time.Date(2026, 8, 5, 3, 0, 0, 0, loc)
	got := Evaluate(CallAttempt{Direction: Inbound, Number: "+15551234567"}, device, now)
	if got != Allow {
		t.Fatalf("expected Allow with no schedules configured, got %v", got)
	}
}
