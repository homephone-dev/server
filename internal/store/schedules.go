package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Schedule struct {
	ID        uuid.UUID
	DeviceID  uuid.UUID
	DayOfWeek *int16
	StartTime time.Duration // offset since midnight, stored as Postgres TIME
	EndTime   time.Duration
	Mode      string
	CreatedAt time.Time
}

func (db *DB) CreateSchedule(ctx context.Context, s Schedule) (Schedule, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO schedules (device_id, day_of_week, start_time, end_time, mode)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, device_id, day_of_week, start_time, end_time, mode, created_at`,
		s.DeviceID, s.DayOfWeek, durationToPGTime(s.StartTime), durationToPGTime(s.EndTime), s.Mode)
	return scanSchedule(row)
}

func (db *DB) GetSchedule(ctx context.Context, id uuid.UUID) (Schedule, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, device_id, day_of_week, start_time, end_time, mode, created_at
		FROM schedules WHERE id = $1`, id)
	return scanSchedule(row)
}

func (db *DB) ListSchedules(ctx context.Context, deviceID uuid.UUID) ([]Schedule, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, device_id, day_of_week, start_time, end_time, mode, created_at
		FROM schedules WHERE device_id = $1 ORDER BY created_at`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Schedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateSchedule updates a schedule. hasDayOfWeek distinguishes "leave
// day_of_week unchanged" from "set it to NULL (every day)", since a nil
// *int16 is ambiguous between those two cases with plain COALESCE.
func (db *DB) UpdateSchedule(ctx context.Context, id uuid.UUID, dayOfWeek *int16, hasDayOfWeek bool, start, end *time.Duration, mode *string) (Schedule, error) {
	var startPG, endPG *time.Time
	if start != nil {
		t := durationToPGTime(*start)
		startPG = &t
	}
	if end != nil {
		t := durationToPGTime(*end)
		endPG = &t
	}
	row := db.Pool.QueryRow(ctx, `
		UPDATE schedules SET
			day_of_week = CASE WHEN $6 THEN $2 ELSE day_of_week END,
			start_time = COALESCE($3, start_time),
			end_time = COALESCE($4, end_time),
			mode = COALESCE($5, mode)
		WHERE id = $1
		RETURNING id, device_id, day_of_week, start_time, end_time, mode, created_at`,
		id, dayOfWeek, startPG, endPG, mode, hasDayOfWeek)
	return scanSchedule(row)
}

func (db *DB) DeleteSchedule(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM schedules WHERE id = $1`, id)
	return err
}

func durationToPGTime(d time.Duration) time.Time {
	return time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(d)
}

func scanSchedule(row rowScanner) (Schedule, error) {
	var s Schedule
	var start, end time.Time
	err := row.Scan(&s.ID, &s.DeviceID, &s.DayOfWeek, &start, &end, &s.Mode, &s.CreatedAt)
	if err != nil {
		return Schedule{}, err
	}
	s.StartTime = time.Duration(start.Hour())*time.Hour + time.Duration(start.Minute())*time.Minute + time.Duration(start.Second())*time.Second
	s.EndTime = time.Duration(end.Hour())*time.Hour + time.Duration(end.Minute())*time.Minute + time.Duration(end.Second())*time.Second
	return s, nil
}
