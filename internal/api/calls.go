package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/zfand/homephone-dev-server/internal/store"
)

type callDTO struct {
	ID              string     `json:"id"`
	DeviceID        string     `json:"deviceId"`
	Direction       string     `json:"direction"`
	RemoteNumber    string     `json:"remoteNumber"`
	StartedAt       time.Time  `json:"startedAt"`
	AnsweredAt      *time.Time `json:"answeredAt"`
	EndedAt         *time.Time `json:"endedAt"`
	DurationSeconds *int       `json:"durationSeconds"`
	Outcome         string     `json:"outcome"`
	Reason          *string    `json:"reason"`
}

func toCallDTO(c store.CallLog) callDTO {
	return callDTO{
		ID: c.ID.String(), DeviceID: c.DeviceID.String(), Direction: c.Direction, RemoteNumber: c.RemoteNumber,
		StartedAt: c.StartedAt, AnsweredAt: c.AnsweredAt, EndedAt: c.EndedAt,
		DurationSeconds: c.DurationSeconds, Outcome: c.Outcome, Reason: c.Reason,
	}
}

// listCalls is GET /devices/{id}/calls?since=&limit= — paginated, read-only.
func (s *Server) listCalls(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	var since *time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since (must be RFC3339)")
			return
		}
		since = &t
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	calls, err := s.db.ListCallLogs(r.Context(), deviceID, since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]callDTO, len(calls))
	for i, c := range calls {
		out[i] = toCallDTO(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getCall(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "call id")
	if !ok {
		return
	}
	c, err := s.db.GetCallLog(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "call not found")
		return
	}
	writeJSON(w, http.StatusOK, toCallDTO(c))
}

// deleteCall is DELETE /calls/{id} — single-record PII deletion (primitive #4).
func (s *Server) deleteCall(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "call id")
	if !ok {
		return
	}
	if err := s.db.DeleteCallLog(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// purgeCalls is DELETE /devices/{id}/calls?number=&before= — targeted
// erasure by number (primitive #2) and/or retention purge by age
// (primitive #3). At least one of the two query parameters is required.
func (s *Server) purgeCalls(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	number := r.URL.Query().Get("number")
	beforeStr := r.URL.Query().Get("before")
	if number == "" && beforeStr == "" {
		writeError(w, http.StatusBadRequest, "must supply number and/or before query parameter")
		return
	}

	var total int64
	if number != "" {
		n, err := s.db.DeleteCallLogsByNumber(r.Context(), deviceID, number)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		total += n
	}
	if beforeStr != "" {
		before, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid before (must be RFC3339)")
			return
		}
		n, err := s.db.DeleteCallLogsBefore(r.Context(), deviceID, before)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		total += n
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": total})
}
