package api

import (
	"net/http"

	"github.com/unidisk/unidisk/internal/store"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	set, err := s.store.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load settings")
		return
	}
	writeJSON(w, http.StatusOK, set)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var set store.Settings
	if !decodeJSON(w, r, &set) {
		return
	}
	// Clamp to a sane range. 100 effectively disables the soft cap (always
	// round-robin); a tiny value pushes everything to the fallback.
	if set.FillThresholdPct < 1 {
		set.FillThresholdPct = 1
	}
	if set.FillThresholdPct > 100 {
		set.FillThresholdPct = 100
	}
	if err := s.store.UpdateSettings(r.Context(), set); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save settings")
		return
	}
	writeJSON(w, http.StatusOK, set)
}
