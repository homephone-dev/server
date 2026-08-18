package api

import (
	"net/http"
	"time"

	"github.com/zfand/homephone-dev-server/internal/store"
)

type contactDTO struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"deviceId"`
	Label     string    `json:"label"`
	Number    string    `json:"number"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toContactDTO(c store.Contact) contactDTO {
	return contactDTO{ID: c.ID.String(), DeviceID: c.DeviceID.String(), Label: c.Label, Number: c.Number, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func (s *Server) listContacts(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	contacts, err := s.db.ListContacts(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]contactDTO, len(contacts))
	for i, c := range contacts {
		out[i] = toContactDTO(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createContact(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	body, ok := decodeJSON[struct {
		Label  string `json:"label"`
		Number string `json:"number"`
	}](w, r)
	if !ok {
		return
	}
	if body.Number == "" {
		writeError(w, http.StatusBadRequest, "number is required")
		return
	}
	c, err := s.db.CreateContact(r.Context(), store.Contact{DeviceID: deviceID, Label: body.Label, Number: body.Number})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toContactDTO(c))
}

func (s *Server) updateContact(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "contact id")
	if !ok {
		return
	}
	body, ok := decodeJSON[struct {
		Label  *string `json:"label"`
		Number *string `json:"number"`
	}](w, r)
	if !ok {
		return
	}
	if body.Number != nil && *body.Number == "" {
		writeError(w, http.StatusBadRequest, "number cannot be empty")
		return
	}
	c, err := s.db.UpdateContact(r.Context(), id, body.Label, body.Number)
	if err != nil {
		writeError(w, http.StatusNotFound, "contact not found")
		return
	}
	writeJSON(w, http.StatusOK, toContactDTO(c))
}

func (s *Server) deleteContact(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "contact id")
	if !ok {
		return
	}
	if err := s.db.DeleteContact(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
