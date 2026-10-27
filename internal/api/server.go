package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"subrefine/internal/auth"
	"subrefine/internal/config"
	"subrefine/internal/core/emitter"
	"subrefine/internal/core/model"
)

// SubManager is the interface for retrieving subscription nodes and verifying sub tokens.
type SubManager interface {
	GetSubscription(id string, token string) ([]*model.ProxyNode, error)
}

// Target formats
const (
	TargetSingbox = "sing-box"
	TargetClash   = "clash"
	TargetURI     = "uri"
)

type Server struct {
	cfg        *config.Config
	subManager SubManager
	mux        *http.ServeMux
}

func NewServer(cfg *config.Config, sm SubManager) *Server {
	s := &Server{
		cfg:        cfg,
		subManager: sm,
		mux:        http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /s/{id}", s.handleSubscribe)
	s.mux.HandleFunc("GET /api/admin/info", s.adminAuthMiddleware(s.handleAdminInfo))
}

func (s *Server) adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !auth.ValidateAdminToken(authHeader, s.cfg.AdminToken) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleAdminInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"version": "1.0.0",
		"host":    s.cfg.Host,
		"port":    s.cfg.Port,
	})
}

// DetectTarget negotiates the target format based on query parameter 'target' or client User-Agent.
func DetectTarget(r *http.Request) string {
	qTarget := strings.ToLower(r.URL.Query().Get("target"))
	switch qTarget {
	case "sing-box", "singbox", "sb":
		return TargetSingbox
	case "clash", "clash.meta", "mihomo":
		return TargetClash
	case "uri", "base64", "v2ray":
		return TargetURI
	}

	ua := strings.ToLower(r.Header.Get("User-Agent"))
	if strings.Contains(ua, "sing-box") || strings.Contains(ua, "box") {
		return TargetSingbox
	}
	if strings.Contains(ua, "clash") || strings.Contains(ua, "mihomo") || strings.Contains(ua, "meta") {
		return TargetClash
	}

	// Default fallback to standard base64 URI list
	return TargetURI
}

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	subID := r.PathValue("id")
	token := r.URL.Query().Get("token")
	if token == "" {
		// Allow token passed in Authorization: Bearer <token>
		if parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2); len(parts) == 2 {
			token = parts[1]
		}
	}

	nodes, err := s.subManager.GetSubscription(subID, token)
	if err != nil {
		http.Error(w, `{"error":"invalid subscription or token"}`, http.StatusForbidden)
		return
	}

	target := DetectTarget(r)

	switch target {
	case TargetSingbox:
		data, err := emitter.ToSingboxJSON(nodes)
		if err != nil {
			http.Error(w, `{"error":"generate singbox config failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)

	case TargetClash:
		data, err := emitter.ToClashYAML(nodes)
		if err != nil {
			http.Error(w, `{"error":"generate clash config failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.Write(data)

	default: // TargetURI (Base64)
		var sb strings.Builder
		for _, n := range nodes {
			u, err := emitter.ToURI(n)
			if err == nil {
				sb.WriteString(u)
				sb.WriteString("\n")
			}
		}
		b64 := base64.StdEncoding.EncodeToString([]byte(sb.String()))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(b64))
	}
}
