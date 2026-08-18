-- +goose Up
CREATE TABLE call_logs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id         UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    direction         TEXT NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    remote_number     TEXT NOT NULL,
    started_at        TIMESTAMPTZ NOT NULL,
    answered_at       TIMESTAMPTZ NULL,
    ended_at          TIMESTAMPTZ NULL,
    duration_seconds  INT NULL,
    outcome           TEXT NOT NULL CHECK (outcome IN ('connected', 'blocked', 'missed', 'failed')),
    reason            TEXT NULL,
    ari_channel_id    TEXT NOT NULL
);

CREATE INDEX idx_call_logs_device_id ON call_logs(device_id);
CREATE INDEX idx_call_logs_device_started ON call_logs(device_id, started_at);
CREATE INDEX idx_call_logs_remote_number ON call_logs(device_id, remote_number);

-- +goose Down
DROP TABLE call_logs;
