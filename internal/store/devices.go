package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Device struct {
	ID            uuid.UUID
	Name          string
	SIPEndpointID string
	Timezone      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (db *DB) CreateDevice(ctx context.Context, d Device) (Device, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO devices (name, sip_endpoint_id, timezone)
		VALUES ($1, $2, $3)
		RETURNING id, name, sip_endpoint_id, timezone, created_at, updated_at`,
		d.Name, d.SIPEndpointID, d.Timezone)
	return scanDevice(row)
}

func (db *DB) GetDevice(ctx context.Context, id uuid.UUID) (Device, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, name, sip_endpoint_id, timezone, created_at, updated_at
		FROM devices WHERE id = $1`, id)
	return scanDevice(row)
}

func (db *DB) GetDeviceBySIPEndpoint(ctx context.Context, sipEndpointID string) (Device, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, name, sip_endpoint_id, timezone, created_at, updated_at
		FROM devices WHERE sip_endpoint_id = $1`, sipEndpointID)
	return scanDevice(row)
}

func (db *DB) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, name, sip_endpoint_id, timezone, created_at, updated_at
		FROM devices ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (db *DB) UpdateDevice(ctx context.Context, id uuid.UUID, name, timezone *string) (Device, error) {
	row := db.Pool.QueryRow(ctx, `
		UPDATE devices SET
			name = COALESCE($2, name),
			timezone = COALESCE($3, timezone),
			updated_at = now()
		WHERE id = $1
		RETURNING id, name, sip_endpoint_id, timezone, created_at, updated_at`,
		id, name, timezone)
	return scanDevice(row)
}

// DeleteDevice deletes a device. Its contacts, schedules, and call_logs are
// purged via ON DELETE CASCADE foreign keys defined in the migrations.
func (db *DB) DeleteDevice(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM devices WHERE id = $1`, id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDevice(row rowScanner) (Device, error) {
	var d Device
	err := row.Scan(&d.ID, &d.Name, &d.SIPEndpointID, &d.Timezone, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}
