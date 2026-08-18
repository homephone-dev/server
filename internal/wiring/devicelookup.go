// Package wiring adapts internal/store to the interfaces internal/callctl
// expects (DeviceLookup, CallLogger), keeping store and callctl from
// depending on each other directly.
package wiring

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/zfand/homephone-dev-server/internal/callctl"
	"github.com/zfand/homephone-dev-server/internal/decision"
	"github.com/zfand/homephone-dev-server/internal/store"
)

type DeviceLookup struct {
	DB *store.DB
}

func (l *DeviceLookup) DeviceBySIPEndpoint(ctx context.Context, endpoint string) (callctl.DeviceRecord, error) {
	d, err := l.DB.GetDeviceBySIPEndpoint(ctx, endpoint)
	if err != nil {
		return callctl.DeviceRecord{}, fmt.Errorf("wiring: no device with sip endpoint %q: %w", endpoint, err)
	}
	return l.assemble(ctx, d)
}

func (l *DeviceLookup) DeviceByID(ctx context.Context, id uuid.UUID) (callctl.DeviceRecord, error) {
	d, err := l.DB.GetDevice(ctx, id)
	if err != nil {
		return callctl.DeviceRecord{}, err
	}
	return l.assemble(ctx, d)
}

func (l *DeviceLookup) assemble(ctx context.Context, d store.Device) (callctl.DeviceRecord, error) {
	loc, err := time.LoadLocation(d.Timezone)
	if err != nil {
		return callctl.DeviceRecord{}, fmt.Errorf("invalid timezone %q for device %s: %w", d.Timezone, d.ID, err)
	}

	contacts, err := l.DB.ListContacts(ctx, d.ID)
	if err != nil {
		return callctl.DeviceRecord{}, err
	}
	allowlist := make([]string, len(contacts))
	for i, c := range contacts {
		allowlist[i] = c.Number
	}

	schedules, err := l.DB.ListSchedules(ctx, d.ID)
	if err != nil {
		return callctl.DeviceRecord{}, err
	}
	windows := make([]decision.ScheduleWindow, len(schedules))
	for i, sc := range schedules {
		var dow *int
		if sc.DayOfWeek != nil {
			day := int(*sc.DayOfWeek)
			dow = &day
		}
		windows[i] = decision.ScheduleWindow{
			DayOfWeek: dow,
			Start:     sc.StartTime,
			End:       sc.EndTime,
			Mode:      decision.Outcome(sc.Mode),
		}
	}

	return callctl.DeviceRecord{
		ID:        d.ID,
		Timezone:  loc,
		Allowlist: allowlist,
		Schedules: windows,
		Endpoint:  d.SIPEndpointID,
	}, nil
}
