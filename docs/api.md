# REST API

Base path: `/api/v1`. Every route requires `Authorization: Bearer <API_TOKEN>`.
Missing or wrong tokens get `401 Unauthorized`.

## Devices

| Method | Path | Notes |
|---|---|---|
| GET | `/devices` | list |
| POST | `/devices` | `{name, sipEndpointId, timezone}` |
| GET | `/devices/{id}` | |
| PATCH | `/devices/{id}` | `{name?, timezone?}` |
| DELETE | `/devices/{id}` | cascades — see Data retention below |

Device JSON: `{id, name, sipEndpointId, timezone, createdAt, updatedAt}`.

## Contacts (per-device allowlist entries)

| Method | Path | Notes |
|---|---|---|
| GET | `/devices/{id}/contacts` | list |
| POST | `/devices/{id}/contacts` | `{label, number}` |
| PATCH | `/contacts/{id}` | `{label?, number?}` |
| DELETE | `/contacts/{id}` | |

Contact JSON: `{id, deviceId, label, number, createdAt, updatedAt}`.

## Schedules

| Method | Path | Notes |
|---|---|---|
| GET | `/devices/{id}/schedules` | list |
| POST | `/devices/{id}/schedules` | `{dayOfWeek?, startTime, endTime, mode}` |
| PATCH | `/schedules/{id}` | `{dayOfWeek?, hasDayOfWeek?, startTime?, endTime?, mode?}` |
| DELETE | `/schedules/{id}` | |

Schedule JSON: `{id, deviceId, dayOfWeek, startTime, endTime, mode}`.
`dayOfWeek` is 0 (Sunday) through 6 (Saturday), or `null` for every day.
`startTime`/`endTime` are `HH:MM:SS` (24h, local to the device's timezone).
`mode` is `"allow"` or `"block"`.

On PATCH, set `hasDayOfWeek: true` together with `dayOfWeek: null` to
explicitly clear a day restriction (make it apply every day); omitting
`hasDayOfWeek` leaves the existing `dayOfWeek` untouched.

## Calls (call log / CDR)

| Method | Path | Notes |
|---|---|---|
| GET | `/devices/{id}/calls?since=&limit=` | paginated, read-only, newest first |
| GET | `/calls/{id}` | |
| DELETE | `/calls/{id}` | delete one call log row |
| DELETE | `/devices/{id}/calls?number=&before=` | erasure/purge, see below |

Call JSON: `{id, deviceId, direction, remoteNumber, startedAt, answeredAt, endedAt, durationSeconds, outcome, reason}`.
`direction` is `"inbound"` or `"outbound"`. `outcome` is one of `"connected"`,
`"blocked"`, `"missed"`, `"failed"`. `since` (query param) is RFC3339.

## Data retention / PII deletion

`contacts.number` and `call_logs.remote_number` are personally identifiable
information; `call_logs` as a whole is call-detail-record-style data. PII
deletion is structural, not just an API convenience:

1. **Cascade on device delete.** `DELETE /devices/{id}` cascades via a real
   `ON DELETE CASCADE` foreign-key constraint (defined in the migrations, not
   emulated in application code) to purge that device's contacts, schedules,
   and call_logs entirely.
2. **Targeted erasure by number.** `DELETE /devices/{id}/calls?number=<E.164>`
   deletes every call_log row for that device whose `remote_number` matches —
   lets someone erase all history for one contact without deleting the
   device itself.
3. **Retention purge by age.** `DELETE /devices/{id}/calls?before=<RFC3339>`
   deletes call_log rows with `started_at` before the given timestamp. The
   two query parameters (`number`, `before`) can be combined; at least one is
   required.
4. **Single-record delete.** `DELETE /calls/{id}` removes exactly one
   call_log row.

All four are implemented in `internal/store/calllogs.go` and exposed via
`internal/api/calls.go`.
