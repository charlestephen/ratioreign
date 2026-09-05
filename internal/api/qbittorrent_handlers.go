package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/qbittorrent"
)

type qbTestRequest struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type qbTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// handleTestQBittorrent runs a real login against qBittorrent's WebUI API
// with the submitted (not-yet-saved) credentials and reports back exactly
// what qBittorrent said — the HTTP status and response body, which is
// usually enough on its own to tell a wrong password apart from a blocked
// connection (timeout), a firewalled network (connection refused), or
// qBittorrent's Host header validation rejecting the request (a bare 401
// with no login-specific body). That's the fastest way to debug a qBittorrent
// sync that "won't sign in" without shelling into the container to read logs.
func (s *Server) handleTestQBittorrent(w http.ResponseWriter, r *http.Request) {
	var req qbTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	// A blank password means "use the currently saved one", mirroring the
	// config-save semantics, so testing doesn't force re-entering it.
	if req.Password == "" && s.Config.QBittorrent != nil {
		req.Password = s.Config.QBittorrent.Password
	}

	client, err := qbittorrent.NewClient(req.URL, req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusOK, qbTestResult{Message: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := client.Login(ctx); err != nil {
		writeJSON(w, http.StatusOK, qbTestResult{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, qbTestResult{Success: true, Message: "Login succeeded."})
}

func (s *Server) handleQBittorrentStatus(w http.ResponseWriter, r *http.Request) {
	if s.QBittorrentSync == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	status := s.QBittorrentSync.Status()
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":         true,
		"lastPollAt":      status.LastPollAt,
		"lastError":       status.LastError,
		"torrentsTracked": status.Tracked,
	})
}
