package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Contact struct {
	ID        uuid.UUID
	DeviceID  uuid.UUID
	Label     string
	Number    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (db *DB) CreateContact(ctx context.Context, c Contact) (Contact, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO contacts (device_id, label, number)
		VALUES ($1, $2, $3)
		RETURNING id, device_id, label, number, created_at, updated_at`,
		c.DeviceID, c.Label, c.Number)
	return scanContact(row)
}

func (db *DB) GetContact(ctx context.Context, id uuid.UUID) (Contact, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, device_id, label, number, created_at, updated_at
		FROM contacts WHERE id = $1`, id)
	return scanContact(row)
}

func (db *DB) ListContacts(ctx context.Context, deviceID uuid.UUID) ([]Contact, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, device_id, label, number, created_at, updated_at
		FROM contacts WHERE device_id = $1 ORDER BY created_at`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Contact
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) UpdateContact(ctx context.Context, id uuid.UUID, label, number *string) (Contact, error) {
	row := db.Pool.QueryRow(ctx, `
		UPDATE contacts SET
			label = COALESCE($2, label),
			number = COALESCE($3, number),
			updated_at = now()
		WHERE id = $1
		RETURNING id, device_id, label, number, created_at, updated_at`,
		id, label, number)
	return scanContact(row)
}

func (db *DB) DeleteContact(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM contacts WHERE id = $1`, id)
	return err
}

func scanContact(row rowScanner) (Contact, error) {
	var c Contact
	err := row.Scan(&c.ID, &c.DeviceID, &c.Label, &c.Number, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
