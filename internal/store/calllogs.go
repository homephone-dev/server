package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CallLog struct {
	ID               uuid.UUID
	DeviceID         uuid.UUID
	Direction        string
	RemoteNumber     string
	StartedAt        time.Time
	AnsweredAt       *time.Time
	EndedAt          *time.Time
	DurationSeconds  *int
	Outcome          string
	Reason           *string
	AriChannelID     string
}

func (db *DB) CreateCallLog(ctx context.Context, c CallLog) (CallLog, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO call_logs (device_id, direction, remote_number, started_at, answered_at, ended_at, duration_seconds, outcome, reason, ari_channel_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, device_id, direction, remote_number, started_at, answered_at, ended_at, duration_seconds, outcome, reason, ari_channel_id`,
		c.DeviceID, c.Direction, c.RemoteNumber, c.StartedAt, c.AnsweredAt, c.EndedAt, c.DurationSeconds, c.Outcome, c.Reason, c.AriChannelID)
	return scanCallLog(row)
}

func (db *DB) UpdateCallLogByChannel(ctx context.Context, ariChannelID string, answeredAt, endedAt *time.Time, durationSeconds *int, outcome *string, reason *string) (CallLog, error) {
	row := db.Pool.QueryRow(ctx, `
		UPDATE call_logs SET
			answered_at = COALESCE($2, answered_at),
			ended_at = COALESCE($3, ended_at),
			duration_seconds = COALESCE($4, duration_seconds),
			outcome = COALESCE($5, outcome),
			reason = COALESCE($6, reason)
		WHERE ari_channel_id = $1
		RETURNING id, device_id, direction, remote_number, started_at, answered_at, ended_at, duration_seconds, outcome, reason, ari_channel_id`,
		ariChannelID, answeredAt, endedAt, durationSeconds, outcome, reason)
	return scanCallLog(row)
}

func (db *DB) GetCallLog(ctx context.Context, id uuid.UUID) (CallLog, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, device_id, direction, remote_number, started_at, answered_at, ended_at, duration_seconds, outcome, reason, ari_channel_id
		FROM call_logs WHERE id = $1`, id)
	return scanCallLog(row)
}

func (db *DB) ListCallLogs(ctx context.Context, deviceID uuid.UUID, since *time.Time, limit int) ([]CallLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT id, device_id, direction, remote_number, started_at, answered_at, ended_at, duration_seconds, outcome, reason, ari_channel_id
		FROM call_logs
		WHERE device_id = $1 AND ($2::timestamptz IS NULL OR started_at >= $2)
		ORDER BY started_at DESC
		LIMIT $3`, deviceID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CallLog
	for rows.Next() {
		c, err := scanCallLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteCallLog removes a single call_log row (PII deletion primitive #4).
func (db *DB) DeleteCallLog(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM call_logs WHERE id = $1`, id)
	return err
}

// DeleteCallLogsByNumber deletes all call_log rows for a device matching a
// remote number (PII deletion primitive #2: targeted erasure by number).
func (db *DB) DeleteCallLogsByNumber(ctx context.Context, deviceID uuid.UUID, number string) (int64, error) {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM call_logs WHERE device_id = $1 AND remote_number = $2`, deviceID, number)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteCallLogsBefore deletes call_log rows older than the given time
// (PII deletion primitive #3: retention purge by age).
func (db *DB) DeleteCallLogsBefore(ctx context.Context, deviceID uuid.UUID, before time.Time) (int64, error) {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM call_logs WHERE device_id = $1 AND started_at < $2`, deviceID, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func scanCallLog(row rowScanner) (CallLog, error) {
	var c CallLog
	err := row.Scan(&c.ID, &c.DeviceID, &c.Direction, &c.RemoteNumber, &c.StartedAt, &c.AnsweredAt, &c.EndedAt, &c.DurationSeconds, &c.Outcome, &c.Reason, &c.AriChannelID)
	return c, err
}
