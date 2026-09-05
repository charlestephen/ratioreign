// Package api exposes RatioReign's REST API and web UI: torrent status and
// control, live config editing, and qBittorrent connectivity diagnostics.
//
// Config edits are not hot-reloaded into the running process — Save just
// writes the YAML file and asks main to restart (see Deps.RequestRestart).
// Every subsystem (the seeder, watchers, qBittorrent sync) is wired up once
// at startup from a single immutable *config.Config, so re-reading it after
// a fresh process start is far simpler and safer than trying to tear down
// and rebuild each subsystem in place while it's mid-flight.
package api

import (
	"encoding/json"
	"net/http"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/qbittorrent"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/seeder"
)

// Deps are the pieces of the running process the API needs access to.
type Deps struct {
	Manager    *seeder.Manager
	Config     *config.Config
	ConfigPath string

	// QBittorrentSync is nil when qBittorrent sync isn't enabled.
	QBittorrentSync *qbittorrent.Sync

	// RequestRestart asks main to shut down cleanly and exit so the
	// container's restart policy brings it back up reading the new config.
	// May be nil (e.g. in tests) — handlers must tolerate that.
	RequestRestart func()
}

type Server struct {
	Deps
}

func New(d Deps) http.Handler {
	s := &Server{Deps: d}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /api/torrents", s.handleListTorrents)
	mux.HandleFunc("POST /api/torrents", s.handleUploadTorrent)
	mux.HandleFunc("DELETE /api/torrents/{hash}", s.handleRemoveTorrent)

	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	mux.HandleFunc("GET /api/profiles", s.handleListProfiles)

	mux.HandleFunc("GET /api/qbittorrent/status", s.handleQBittorrentStatus)
	mux.HandleFunc("POST /api/qbittorrent/test", s.handleTestQBittorrent)

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleListTorrents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Manager.Snapshot())
}

func (s *Server) handleRemoveTorrent(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	s.Manager.Remove(hash)
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
