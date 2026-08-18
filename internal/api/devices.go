package api

import (
	"net/http"
	"time"

	"github.com/zfand/homephone-dev-server/internal/store"
)

type deviceDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	SIPEndpointID string    `json:"sipEndpointId"`
	Timezone      string    `json:"timezone"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func toDeviceDTO(d store.Device) deviceDTO {
	return deviceDTO{
		ID:            d.ID.String(),
		Name:          d.Name,
		SIPEndpointID: d.SIPEndpointID,
		Timezone:      d.Timezone,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.db.ListDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]deviceDTO, len(devices))
	for i, d := range devices {
		out[i] = toDeviceDTO(d)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeJSON[struct {
		Name          string `json:"name"`
		SIPEndpointID string `json:"sipEndpointId"`
		Timezone      string `json:"timezone"`
	}](w, r)
	if !ok {
		return
	}
	if body.Name == "" || body.SIPEndpointID == "" {
		writeError(w, http.StatusBadRequest, "name and sipEndpointId are required")
		return
	}
	if body.Timezone == "" {
		body.Timezone = "UTC"
	}
	if !validTimezone(body.Timezone) {
		writeError(w, http.StatusBadRequest, "timezone must be a valid IANA zone name")
		return
	}
	d, err := s.db.CreateDevice(r.Context(), store.Device{Name: body.Name, SIPEndpointID: body.SIPEndpointID, Timezone: body.Timezone})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toDeviceDTO(d))
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	d, err := s.db.GetDevice(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}

func (s *Server) updateDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	body, ok := decodeJSON[struct {
		Name     *string `json:"name"`
		Timezone *string `json:"timezone"`
	}](w, r)
	if !ok {
		return
	}
	if body.Timezone != nil && !validTimezone(*body.Timezone) {
		writeError(w, http.StatusBadRequest, "timezone must be a valid IANA zone name")
		return
	}
	d, err := s.db.UpdateDevice(r.Context(), id, body.Name, body.Timezone)
	if err != nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, toDeviceDTO(d))
}

// deleteDevice cascades to purge the device's contacts, schedules, and
// call_logs via the ON DELETE CASCADE foreign keys in the migrations.
func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	if err := s.db.DeleteDevice(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}
