package api

import (
	"net/http"
	"time"

	"github.com/zfand/homephone-dev-server/internal/store"
)

type scheduleDTO struct {
	ID        string `json:"id"`
	DeviceID  string `json:"deviceId"`
	DayOfWeek *int16 `json:"dayOfWeek"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Mode      string `json:"mode"`
}

func toScheduleDTO(s store.Schedule) scheduleDTO {
	return scheduleDTO{
		ID:        s.ID.String(),
		DeviceID:  s.DeviceID.String(),
		DayOfWeek: s.DayOfWeek,
		StartTime: formatDuration(s.StartTime),
		EndTime:   formatDuration(s.EndTime),
		Mode:      s.Mode,
	}
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	return time.Date(0, 1, 1, h, m, sec, 0, time.UTC).Format("15:04:05")
}

func parseTimeOfDay(s string) (time.Duration, error) {
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		t, err = time.Parse("15:04", s)
		if err != nil {
			return 0, err
		}
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second, nil
}

func (s *Server) listSchedules(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	schedules, err := s.db.ListSchedules(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]scheduleDTO, len(schedules))
	for i, sc := range schedules {
		out[i] = toScheduleDTO(sc)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createSchedule(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := parsePathID(w, r, "device id")
	if !ok {
		return
	}
	body, ok := decodeJSON[struct {
		DayOfWeek *int16 `json:"dayOfWeek"`
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
		Mode      string `json:"mode"`
	}](w, r)
	if !ok {
		return
	}
	if body.Mode != "allow" && body.Mode != "block" {
		writeError(w, http.StatusBadRequest, "mode must be 'allow' or 'block'")
		return
	}
	start, err := parseTimeOfDay(body.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid startTime")
		return
	}
	end, err := parseTimeOfDay(body.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid endTime")
		return
	}
	sc, err := s.db.CreateSchedule(r.Context(), store.Schedule{
		DeviceID: deviceID, DayOfWeek: body.DayOfWeek, StartTime: start, EndTime: end, Mode: body.Mode,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toScheduleDTO(sc))
}

func (s *Server) updateSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "schedule id")
	if !ok {
		return
	}
	body, ok := decodeJSON[struct {
		DayOfWeek    *int16  `json:"dayOfWeek"`
		HasDayOfWeek bool    `json:"hasDayOfWeek"`
		StartTime    *string `json:"startTime"`
		EndTime      *string `json:"endTime"`
		Mode         *string `json:"mode"`
	}](w, r)
	if !ok {
		return
	}
	var start, end *time.Duration
	if body.StartTime != nil {
		d, err := parseTimeOfDay(*body.StartTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid startTime")
			return
		}
		start = &d
	}
	if body.EndTime != nil {
		d, err := parseTimeOfDay(*body.EndTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid endTime")
			return
		}
		end = &d
	}
	sc, err := s.db.UpdateSchedule(r.Context(), id, body.DayOfWeek, body.HasDayOfWeek, start, end, body.Mode)
	if err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	writeJSON(w, http.StatusOK, toScheduleDTO(sc))
}

func (s *Server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "schedule id")
	if !ok {
		return
	}
	if err := s.db.DeleteSchedule(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
