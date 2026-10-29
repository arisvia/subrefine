package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/arisvia/subrefine/internal/storage"
)

// adminKey is the console view of a global delivery key: the credential plus
// the subscription ids attaching it, so the UI can show blast radius before
// rotation or deletion.
type adminKey struct {
	*storage.DeliveryKey
	InUse []string `json:"in_use"`
}

func (s *Server) handleAdminListKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	list := s.storage.ListKeys()
	out := make([]adminKey, 0, len(list))
	for _, k := range list {
		out = append(out, adminKey{DeliveryKey: k, InUse: s.storage.KeyUsage(k.ID)})
	}
	json.NewEncoder(w).Encode(out)
}

// handleAdminCreateKey accepts {name, token?}; an empty token is generated.
func (s *Server) handleAdminCreateKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid key body")
		return
	}
	key := &storage.DeliveryKey{
		Name:  strings.TrimSpace(body.Name),
		Token: strings.TrimSpace(body.Token),
	}
	if err := s.storage.SaveKey(key); err != nil {
		if errors.Is(err, storage.ErrTokenTaken) {
			writeJSONError(w, http.StatusConflict, "that token is already used by another key")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "save failed: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(adminKey{DeliveryKey: key, InUse: []string{}})
}

// handleAdminUpdateKey patches name and/or token. Rotating the token changes
// every link on every subscription attaching this key - the console warns.
func (s *Server) handleAdminUpdateKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	key, err := s.storage.GetKey(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "key not found")
		return
	}
	var body struct {
		Name  *string `json:"name"`
		Token *string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid key body")
		return
	}
	patched := *key
	if body.Name != nil {
		patched.Name = strings.TrimSpace(*body.Name)
	}
	if body.Token != nil {
		patched.Token = strings.TrimSpace(*body.Token)
	}
	if err := s.storage.SaveKey(&patched); err != nil {
		if errors.Is(err, storage.ErrTokenTaken) {
			writeJSONError(w, http.StatusConflict, "that token is already used by another key")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "save failed: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(adminKey{DeliveryKey: &patched, InUse: s.storage.KeyUsage(id)})
}

// handleAdminDeleteKey refuses (409 + usage list) while subscriptions attach
// the key unless ?cascade=1 detaches it everywhere. A subscription left with
// no keys gets a fresh default from the store, so it stays deliverable.
func (s *Server) handleAdminDeleteKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.PathValue("id")
	cascade := r.URL.Query().Get("cascade") == "1"
	err := s.storage.DeleteKey(id, cascade)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	case errors.Is(err, storage.ErrNotFound):
		writeJSONError(w, http.StatusNotFound, "key not found")
	case errors.Is(err, storage.ErrInUse):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]any{
			"error":  "key is attached to subscriptions",
			"in_use": s.storage.KeyUsage(id),
		})
	default:
		writeJSONError(w, http.StatusInternalServerError, "delete failed: "+err.Error())
	}
}
