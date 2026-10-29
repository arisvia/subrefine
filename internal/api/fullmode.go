package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/arisvia/subrefine/internal/storage"
)

// resolveFullMode decides the full-config template mode for a delivery:
// the URL parameter overrides the saved subscription mode, and "off"/
// "fragment" always selects the plain fragment form.
func resolveFullMode(r *http.Request, saved string) string {
	m := strings.ToLower(r.URL.Query().Get("mode"))
	if m == "" {
		m = strings.ToLower(saved)
	}
	if m == "off" || m == "fragment" {
		return ""
	}
	return m
}

// validateFullModes rejects saved template modes the server cannot serve, so
// a typo in the console surfaces at save time instead of on every client
// fetch. Modes are normalised to lower case here.
func (s *Server) validateFullModes(sub *storage.Subscription) error {
	sub.SingboxMode = strings.ToLower(sub.SingboxMode)
	sub.ClashMode = strings.ToLower(sub.ClashMode)
	if sub.SingboxMode == "" && sub.ClashMode == "" {
		return nil
	}
	if s.templates == nil {
		return errors.New("full-config templates are not configured on this server (set SUBREFINE_TEMPLATES)")
	}
	sb, cl := s.templates.Modes()
	if sub.SingboxMode != "" && !slices.Contains(sb, sub.SingboxMode) {
		return fmt.Errorf("unknown sing-box template mode %q (available: %s)", sub.SingboxMode, strings.Join(sb, ", "))
	}
	if sub.ClashMode != "" && !slices.Contains(cl, sub.ClashMode) {
		return fmt.Errorf("unknown clash template mode %q (available: %s)", sub.ClashMode, strings.Join(cl, ", "))
	}
	return nil
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
