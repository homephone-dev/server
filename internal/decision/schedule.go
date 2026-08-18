package decision

import "time"

// inAllowedWindow determines whether now falls in an "allow" schedule window.
// If no schedules are configured at all, calls are allowed at any time
// (schedule is opt-in restriction on top of the allowlist). If schedules ARE
// configured, the call is allowed only if it falls in an "allow" window and
// not in an overlapping "block" window that takes precedence.
func inAllowedWindow(schedules []ScheduleWindow, loc *time.Location, now time.Time) bool {
	if len(schedules) == 0 {
		return true
	}
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	dow := int(local.Weekday())
	offset := time.Duration(local.Hour())*time.Hour +
		time.Duration(local.Minute())*time.Minute +
		time.Duration(local.Second())*time.Second

	allowed := false
	for _, w := range schedules {
		if w.DayOfWeek != nil && *w.DayOfWeek != dow {
			continue
		}
		inWindow := false
		if w.Start <= w.End {
			inWindow = offset >= w.Start && offset < w.End
		} else {
			// Wraparound window spanning midnight (e.g. 22:00-06:00).
			inWindow = offset >= w.Start || offset < w.End
		}
		if !inWindow {
			continue
		}
		if w.Mode == Block {
			return false // explicit block window takes precedence
		}
		if w.Mode == Allow {
			allowed = true
		}
	}
	return allowed
}
