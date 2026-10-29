package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// resolveFullMode decides the full-config template mode for a delivery from
// the URL alone: the console's link dialog bakes the chosen mode into every
// link, so nothing is bound to the subscription itself. "off"/"fragment"
// select the plain fragment form.
func resolveFullMode(r *http.Request) string {
	m := strings.ToLower(r.URL.Query().Get("mode"))
	if m == "off" || m == "fragment" {
		return ""
	}
	return m
}

// handleAdminTemplates lists the loadable full-config modes per family so the
// console can offer them as choices; empty arrays mean the feature is off.
func (s *Server) handleAdminTemplates(w http.ResponseWriter, r *http.Request) {
	sb, cl := []string{}, []string{}
	if s.templates != nil {
		var a, b []string
		a, b = s.templates.Modes()
		if a != nil {
			sb = a
		}
		if b != nil {
			cl = b
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"singbox": sb, "clash": cl})
}
