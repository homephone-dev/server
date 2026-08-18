// Package api implements the REST API (stdlib net/http, Go 1.22+ ServeMux
// pattern routing, no router dependency), protected by a bearer-token
// middleware checking Authorization: Bearer <API_TOKEN> on every request.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/zfand/homephone-dev-server/internal/store"
)

type Server struct {
	db       *store.DB
	apiToken string
	logger   *slog.Logger
	mux      *http.ServeMux
}

func NewServer(db *store.DB, apiToken string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{db: db, apiToken: apiToken, logger: logger, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.withAuth(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/devices", s.listDevices)
	s.mux.HandleFunc("POST /api/v1/devices", s.createDevice)
	s.mux.HandleFunc("GET /api/v1/devices/{id}", s.getDevice)
	s.mux.HandleFunc("PATCH /api/v1/devices/{id}", s.updateDevice)
	s.mux.HandleFunc("DELETE /api/v1/devices/{id}", s.deleteDevice)

	s.mux.HandleFunc("GET /api/v1/devices/{id}/contacts", s.listContacts)
	s.mux.HandleFunc("POST /api/v1/devices/{id}/contacts", s.createContact)
	s.mux.HandleFunc("PATCH /api/v1/contacts/{id}", s.updateContact)
	s.mux.HandleFunc("DELETE /api/v1/contacts/{id}", s.deleteContact)

	s.mux.HandleFunc("GET /api/v1/devices/{id}/schedules", s.listSchedules)
	s.mux.HandleFunc("POST /api/v1/devices/{id}/schedules", s.createSchedule)
	s.mux.HandleFunc("PATCH /api/v1/schedules/{id}", s.updateSchedule)
	s.mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.deleteSchedule)

	s.mux.HandleFunc("GET /api/v1/devices/{id}/calls", s.listCalls)
	s.mux.HandleFunc("DELETE /api/v1/devices/{id}/calls", s.purgeCalls)
	s.mux.HandleFunc("GET /api/v1/calls/{id}", s.getCall)
	s.mux.HandleFunc("DELETE /api/v1/calls/{id}", s.deleteCall)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parsePathID parses the {id} path value as a UUID, writing a 400 response
// (using label, e.g. "device id") and returning ok=false if it isn't one.
func parsePathID(w http.ResponseWriter, r *http.Request, label string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+label)
		return uuid.UUID{}, false
	}
	return id, true
}

// decodeJSON decodes the request body as T, writing a 400 response and
// returning ok=false if the body isn't valid JSON.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return v, false
	}
	return v, true
}
