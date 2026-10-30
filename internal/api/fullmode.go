package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/arisvia/subrefine/internal/core/emitter"
)

// resolveFull reads the full-config switches from the URL. The console's link
// dialog bakes them into every link, so nothing is bound to the subscription:
//
//	&full=1                 request a complete config instead of the fragment
//	&inbounds=mixed,tun     which inbounds to keep (absent => every one the
//	                        template declares)
//	&ipv6=1|0               force the IPv6 posture (absent => template default)
//
// want is false for the plain fragment form.
func resolveFull(r *http.Request) (want bool, inbounds []string, ipv6 *bool) {
	q := r.URL.Query()
	switch strings.ToLower(q.Get("full")) {
	case "1", "true", "yes":
		want = true
	}
	if !want {
		return
	}
	if ib := q.Get("inbounds"); ib != "" {
		for _, s := range strings.Split(ib, ",") {
			if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
				inbounds = append(inbounds, s)
			}
		}
	}
	if v := q.Get("ipv6"); v != "" {
		b := v == "1" || strings.EqualFold(v, "true")
		ipv6 = &b
	}
	return
}

// fullOptionsFor fills the "keep everything the template declares" default
// when the URL omitted &inbounds, so a bare &full=1 still yields a loadable
// config instead of one with every managed inbound stripped.
func fullOptionsFor(tmplInbounds []string, sel []string, ipv6 *bool) emitter.FullOptions {
	if sel == nil {
		sel = tmplInbounds
	}
	return emitter.FullOptions{Inbounds: sel, IPv6: ipv6}
}

// handleAdminTemplates reports, per family, whether a full-config template is
// loaded and which inbound kinds it declares, so the console can render the
// right switches. Empty means the feature is off.
func (s *Server) handleAdminTemplates(w http.ResponseWriter, r *http.Request) {
	sb := emitter.FamilyOptions{Inbounds: []string{}}
	cl := emitter.FamilyOptions{Inbounds: []string{}}
	if s.templates != nil {
		a, b := s.templates.Options()
		if a.Inbounds == nil {
			a.Inbounds = []string{}
		}
		if b.Inbounds == nil {
			b.Inbounds = []string{}
		}
		sb, cl = a, b
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"singbox": sb, "clash": cl})
}
