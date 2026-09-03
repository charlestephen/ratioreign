// Package api exposes a small JSON REST API for status and control, in
// place of Joal's WebSocket/STOMP UI backend — a plain JSON API is simpler
// to script against and enough for a headless ratio-farming daemon.
package api

import (
	"encoding/json"
	"net/http"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/seeder"
)

type Server struct {
	Manager *seeder.Manager
}

func New(mgr *seeder.Manager) http.Handler {
	s := &Server{Manager: mgr}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/torrents", s.handleListTorrents)
	mux.HandleFunc("DELETE /api/torrents/{hash}", s.handleRemoveTorrent)
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
