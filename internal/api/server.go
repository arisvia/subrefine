package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/arisvia/subrefine/internal/auth"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/emitter"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/core/pipeline"
	"github.com/arisvia/subrefine/internal/storage"
)

type SubStorage interface {
	Get(id string) (*storage.Subscription, error)
	List() []*storage.Subscription
	Save(sub *storage.Subscription) error
	Delete(id string) error
	ListKeys() []*storage.DeliveryKey
	GetKey(id string) (*storage.DeliveryKey, error)
	SaveKey(k *storage.DeliveryKey) error
	DeleteKey(id string, cascade bool) error
	KeyUsage(id string) []string
}

// SubManager is the interface for retrieving subscription nodes and verifying sub tokens.
type SubManager interface {
	GetSubscription(id string, token string) ([]*model.ProxyNode, error)
	// Resolve authenticates a delivery token against the global key pool and
	// returns the subscription's reference for that key; callers apply
	// ref.Disabled per request.
	Resolve(id, token string) (*storage.Subscription, *storage.SubKeyRef, []*model.ProxyNode, error)
	// Invalidate drops cached delivery results for a subscription, so a deleted
	// subscription's nodes (which include credentials) are not held until the TTL
	// lapses.
	Invalidate(id string)
	// FullModes reports the subscription's saved full-config template modes.
	FullModes(id string) (singbox, clash string)
	// UserInfo returns the merged account metadata (expiry/traffic) plus the
	// per-source breakdown; all nil when no upstream reported any.
	UserInfo(id string) (*model.SubscriptionInfo, []*model.SourceInfo, error)
}

// TemplateRegistry supplies the startup-loaded full-config templates.
type TemplateRegistry interface {
	SingboxTemplate(mode string) *emitter.SingboxTemplate
	ClashTemplate(mode string) *emitter.ClashFullTemplate
	Modes() (singbox, clash []string)
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
	templates  TemplateRegistry
	mux        *http.ServeMux
	limiter    *AuthLimiter
}

func NewServer(cfg *config.Config, sm SubManager, st SubStorage, tpl TemplateRegistry) *Server {
	s := &Server{
		cfg:        cfg,
		subManager: sm,
		storage:    st,
		templates:  tpl,
		mux:        http.NewServeMux(),
		limiter:    NewAuthLimiter(cfg != nil && cfg.TrustProxy),
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
	s.mux.HandleFunc("GET /api/admin/templates", s.adminAuthMiddleware(s.handleAdminTemplates))
	s.mux.HandleFunc("GET /api/admin/keys", s.adminAuthMiddleware(s.handleAdminListKeys))
	s.mux.HandleFunc("POST /api/admin/keys", s.adminAuthMiddleware(s.handleAdminCreateKey))
	s.mux.HandleFunc("PUT /api/admin/keys/{id}", s.adminAuthMiddleware(s.handleAdminUpdateKey))
	s.mux.HandleFunc("DELETE /api/admin/keys/{id}", s.adminAuthMiddleware(s.handleAdminDeleteKey))
	s.mux.HandleFunc("GET /api/admin/subscriptions", s.adminAuthMiddleware(s.handleAdminListSubscriptions))
	s.mux.HandleFunc("POST /api/admin/subscriptions", s.adminAuthMiddleware(s.handleAdminSaveSubscription))
	s.mux.HandleFunc("DELETE /api/admin/subscriptions/{id}", s.adminAuthMiddleware(s.handleAdminDeleteSubscription))
	s.mux.HandleFunc("GET /api/admin/subscriptions/{id}/nodes", s.adminAuthMiddleware(s.handleAdminPreviewNodes))

	// Everything else is the embedded console; unknown paths are 404, never the shell.
	s.mux.Handle("GET /", NewDashboardHandler(s.cfg.DashboardDir))
}

func (s *Server) adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := s.limiter.clientKey(r)
		if wait := s.limiter.retryAfter(key); wait > 0 {
			w.Header().Set("Retry-After", retryAfterHeader(wait))
			http.Error(w, `{"error":"too many failed attempts"}`, http.StatusTooManyRequests)
			return
		}
		if !auth.ValidateAdminToken(r.Header.Get("Authorization"), s.cfg.AdminToken) {
			s.limiter.recordFailure(key)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		s.limiter.clear(key)
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
		"version": Version,
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
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid subscription body")
		return
	}
	// Reject configuration that delivery would otherwise ignore, so a typo in
	// the console cannot turn into a subscription that leaks hidden nodes.
	if err := storage.ValidateID(sub.ID); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := pipeline.ValidateSteps(sub.Pipeline); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validateFullModes(&sub); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.storage.Save(&sub); err != nil {
		// A stale key reference means the console (or a hand-edit) named a
		// delivery key that does not exist; that is operator error, not 500.
		if errors.Is(err, storage.ErrUnknownKey) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "save failed: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "sub": sub})
}

// writeJSONError reports a failure with a correctly escaped message, since
// validation errors quote the operator's own input.
func writeJSONError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (s *Server) handleAdminDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	if s.storage == nil {
		http.Error(w, `{"error":"storage unavailable"}`, http.StatusInternalServerError)
		return
	}
	if err := s.storage.Delete(id); err != nil {
		// The row is always dropped from memory first; a persistence failure
		// means it will resurrect on restart, which is a server error, not a
		// missing subscription. Invalidate either way so credentials held in
		// the delivery cache are never left behind.
		if s.subManager != nil {
			s.subManager.Invalidate(id)
		}
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "delete failed: "+err.Error())
		return
	}
	if s.subManager != nil {
		s.subManager.Invalidate(id)
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
	if !sub.Enabled {
		// Previews of a disabled subscription are a conflict, not a server
		// failure; the overview flags them and moves on.
		writeJSONError(w, http.StatusConflict, "subscription is disabled")
		return
	}

	nodes, err := s.subManager.GetSubscription(id, sub.Token)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch nodes: "+err.Error())
		return
	}

	// uris is index-aligned with nodes: the shareable client link for each node,
	// empty when that node cannot be encoded as a URI.
	uris := make([]string, len(nodes))
	// ids is index-aligned with nodes: the stable per-node identity the console
	// records in a key's disabled list.
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		if uri, err := emitter.ToURI(n); err == nil {
			uris[i] = uri
		}
		ids[i] = n.Identity()
	}

	info, sources, _ := s.subManager.UserInfo(id)

	json.NewEncoder(w).Encode(map[string]any{
		"id":       id,
		"count":    len(nodes),
		"nodes":    nodes,
		"uris":     uris,
		"ids":      ids,
		"userinfo": info,
		"sources":  sources,
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

	_, ref, nodes, err := s.subManager.Resolve(subID, token)
	if err != nil {
		http.Error(w, `{"error":"invalid subscription or token"}`, http.StatusForbidden)
		return
	}
	// Per-key blacklist: the shared upstream cache keeps the full set; only
	// this delivery drops withheld nodes.
	nodes = model.FilterDisabled(nodes, ref.Disabled)

	// Re-emit the airport's account metadata so clients show expiry/traffic.
	// Usage is account-level, so the merged view is served to every key.
	if info, _, err := s.subManager.UserInfo(subID); err == nil && info != nil {
		w.Header().Set("subscription-userinfo",
			"upload="+strconv.FormatInt(info.Upload, 10)+
				"; download="+strconv.FormatInt(info.Download, 10)+
				"; total="+strconv.FormatInt(info.Total, 10)+
				"; expire="+strconv.FormatInt(info.Expire, 10))
		if info.UpdateInterval > 0 {
			w.Header().Set("profile-update-interval", strconv.Itoa(info.UpdateInterval))
		}
	}

	target := DetectTarget(r)

	switch target {
	case TargetSingbox:
		if skipped := emitter.SingboxSkipped(nodes); len(skipped) > 0 {
			w.Header().Set("X-Subrefine-Skipped", "wireguard="+strconv.Itoa(len(skipped)))
			log.Printf("[subrefine] %s: sing-box target dropped %d wireguard node(s) (no wireguard outbound since sing-box 1.13); clash and uri targets still carry them", subID, len(skipped))
		}
		sbMode, _ := s.subManager.FullModes(subID)
		if mode := resolveFullMode(r, sbMode); mode != "" {
			var tmpl *emitter.SingboxTemplate
			if s.templates != nil {
				tmpl = s.templates.SingboxTemplate(mode)
			}
			if tmpl == nil {
				w.Header().Set("X-Subrefine-Full", "unknown-mode")
				log.Printf("[subrefine] %s: sing-box full mode %q has no loaded template, serving fragment", subID, mode)
			} else {
				data, err := tmpl.Render(nodes)
				switch {
				case err == nil:
					w.Header().Set("X-Subrefine-Full", "singbox."+mode)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.Write(data)
					return
				case errors.Is(err, emitter.ErrNoDeliverableNodes):
					w.Header().Set("X-Subrefine-Full", "empty-nodes")
					log.Printf("[subrefine] %s: sing-box full mode %q has no deliverable nodes, serving fragment", subID, mode)
				default:
					log.Printf("[subrefine] %s: sing-box full mode %q render failed: %v", subID, mode, err)
					http.Error(w, `{"error":"render sing-box full config failed"}`, http.StatusInternalServerError)
					return
				}
			}
		}
		data, err := emitter.ToSingboxJSON(nodes)
		if err != nil {
			http.Error(w, `{"error":"generate singbox config failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)

	case TargetClash:
		_, clMode := s.subManager.FullModes(subID)
		if mode := resolveFullMode(r, clMode); mode != "" {
			var tmpl *emitter.ClashFullTemplate
			if s.templates != nil {
				tmpl = s.templates.ClashTemplate(mode)
			}
			if tmpl == nil {
				w.Header().Set("X-Subrefine-Full", "unknown-mode")
				log.Printf("[subrefine] %s: clash full mode %q has no loaded template, serving fragment", subID, mode)
			} else {
				data, err := tmpl.Render(nodes)
				switch {
				case err == nil:
					w.Header().Set("X-Subrefine-Full", "clash."+mode)
					w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
					w.Write(data)
					return
				case errors.Is(err, emitter.ErrNoDeliverableNodes):
					w.Header().Set("X-Subrefine-Full", "empty-nodes")
					log.Printf("[subrefine] %s: clash full mode %q has no deliverable nodes, serving fragment", subID, mode)
				default:
					log.Printf("[subrefine] %s: clash full mode %q render failed: %v", subID, mode, err)
					http.Error(w, `{"error":"render clash full config failed"}`, http.StatusInternalServerError)
					return
				}
			}
		}
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
