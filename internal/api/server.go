package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/arisvia/subrefine/internal/auth"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/emitter"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/storage"
	"github.com/arisvia/subrefine/webui"
)

type SubStorage interface {
	Get(id string) (*storage.Subscription, error)
	List() []*storage.Subscription
	Save(sub *storage.Subscription) error
	Delete(id string) error
}

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
	storage    SubStorage
	mux        *http.ServeMux
}

func NewServer(cfg *config.Config, sm SubManager, st SubStorage) *Server {
	s := &Server{
		cfg:        cfg,
		subManager: sm,
		storage:    st,
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
	s.mux.HandleFunc("GET /api/admin/subscriptions", s.adminAuthMiddleware(s.handleAdminListSubscriptions))
	s.mux.HandleFunc("POST /api/admin/subscriptions", s.adminAuthMiddleware(s.handleAdminSaveSubscription))
	s.mux.HandleFunc("DELETE /api/admin/subscriptions/{id}", s.adminAuthMiddleware(s.handleAdminDeleteSubscription))
	s.mux.HandleFunc("GET /api/admin/subscriptions/{id}/nodes", s.adminAuthMiddleware(s.handleAdminPreviewNodes))

	// Embedded WebUI handler
	distFS, err := fs.Sub(webui.FS, "dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distFS))
		s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			fileServer.ServeHTTP(w, r)
		})
	}
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
func (s *Server) handleAdminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.storage == nil {
		json.NewEncoder(w).Encode([]any{})
		return
	}
	subs := s.storage.List()
	json.NewEncoder(w).Encode(subs)
}

func (s *Server) handleAdminSaveSubscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.storage == nil {
		http.Error(w, `{"error":"storage unavailable"}`, http.StatusInternalServerError)
		return
	}
	var sub storage.Subscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil || sub.ID == "" {
		http.Error(w, `{"error":"invalid subscription body"}`, http.StatusBadRequest)
		return
	}
	if err := s.storage.Save(&sub); err != nil {
		http.Error(w, `{"error":"save failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "sub": sub})
}

func (s *Server) handleAdminDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	if s.storage == nil {
		http.Error(w, `{"error":"storage unavailable"}`, http.StatusInternalServerError)
		return
	}
	if err := s.storage.Delete(id); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

func (s *Server) handleAdminPreviewNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	sub, err := s.storage.Get(id)
	if err != nil {
		http.Error(w, `{"error":"subscription not found"}`, http.StatusNotFound)
		return
	}

	nodes, err := s.subManager.GetSubscription(id, sub.Token)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to fetch nodes: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"id":    id,
		"count": len(nodes),
		"nodes": nodes,
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
