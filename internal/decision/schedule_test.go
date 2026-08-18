package decision

import (
	"testing"
	"time"
)

func at(hour, min int) time.Time {
	return time.Date(2024, 1, 1, hour, min, 0, 0, time.UTC)
}

func TestInAllowedWindow_NoSchedules(t *testing.T) {
	if !inAllowedWindow(nil, time.UTC, at(3, 0)) {
		t.Fatal("expected allowed when no schedules configured")
	}
}

func TestInAllowedWindow_NonWraparoundAllow(t *testing.T) {
	windows := []ScheduleWindow{
		{Start: 9 * time.Hour, End: 17 * time.Hour, Mode: Allow},
	}
	if !inAllowedWindow(windows, time.UTC, at(12, 0)) {
		t.Fatal("expected 12:00 to be within 09:00-17:00 allow window")
	}
	if inAllowedWindow(windows, time.UTC, at(20, 0)) {
		t.Fatal("expected 20:00 to be outside 09:00-17:00 allow window")
	}
}

func TestInAllowedWindow_WraparoundAllow(t *testing.T) {
	windows := []ScheduleWindow{
		{Start: 22 * time.Hour, End: 6 * time.Hour, Mode: Allow},
	}
	if !inAllowedWindow(windows, time.UTC, at(23, 0)) {
		t.Fatal("expected 23:00 to match wraparound 22:00-06:00 allow window")
	}
	if !inAllowedWindow(windows, time.UTC, at(2, 0)) {
		t.Fatal("expected 02:00 to match wraparound 22:00-06:00 allow window")
	}
	if inAllowedWindow(windows, time.UTC, at(12, 0)) {
		t.Fatal("expected 12:00 to NOT match wraparound 22:00-06:00 allow window")
	}
}

func TestInAllowedWindow_WraparoundBlock(t *testing.T) {
	windows := []ScheduleWindow{
		{Start: 0, End: 24 * time.Hour, Mode: Allow},
		{Start: 22 * time.Hour, End: 6 * time.Hour, Mode: Block},
	}
	if inAllowedWindow(windows, time.UTC, at(23, 0)) {
		t.Fatal("expected 23:00 to be blocked by wraparound 22:00-06:00 block window")
	}
	if inAllowedWindow(windows, time.UTC, at(2, 0)) {
		t.Fatal("expected 02:00 to be blocked by wraparound 22:00-06:00 block window")
	}
	if !inAllowedWindow(windows, time.UTC, at(12, 0)) {
		t.Fatal("expected 12:00 to remain allowed outside the wraparound block window")
	}
}

func TestInAllowedWindow_DayOfWeekFilter(t *testing.T) {
	monday := 1
	windows := []ScheduleWindow{
		{DayOfWeek: &monday, Start: 9 * time.Hour, End: 17 * time.Hour, Mode: Allow},
	}
	// 2024-01-01 is a Monday.
	if !inAllowedWindow(windows, time.UTC, at(12, 0)) {
		t.Fatal("expected Monday window to match on a Monday")
	}
	// 2024-01-02 is a Tuesday.
	tuesday := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	if inAllowedWindow(windows, time.UTC, tuesday) {
		t.Fatal("expected Monday-only window to not match on a Tuesday")
	}
}
