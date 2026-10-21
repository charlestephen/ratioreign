package qbittorrent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeQBittorrent serves the handful of WebAPI endpoints Sync uses, with a
// mutable torrent list so a test can change swarm counts between polls.
type fakeQBittorrent struct {
	torrents []TorrentInfo
	exported map[string]int
}

func (f *fakeQBittorrent) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Ok."))
	})
	mux.HandleFunc("/api/v2/torrents/info", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(f.torrents)
	})
	mux.HandleFunc("/api/v2/torrents/trackers", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]TrackerEntry{{URL: "https://tracker.example.org/announce"}})
	})
	mux.HandleFunc("/api/v2/torrents/export", func(w http.ResponseWriter, r *http.Request) {
		f.exported[r.URL.Query().Get("hash")]++
		_, _ = w.Write([]byte("fake-torrent-bytes"))
	})
	return mux
}

func newTestSync(t *testing.T, fake *fakeQBittorrent) (*Sync, string) {
	t.Helper()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)
	client, err := NewClient(srv.URL, "user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	return NewSync(client, dir, "", time.Minute), dir
}

func TestSyncExcludesAndLaterImports(t *testing.T) {
	fake := &fakeQBittorrent{exported: map[string]int{}, torrents: []TorrentInfo{
		{Hash: "h1", Name: "Too Seedy", Tracker: "https://tracker.example.org/announce", NumComplete: 900, NumIncomplete: 5},
		{Hash: "h2", Name: "Good One", Tracker: "https://tracker.example.org/announce", NumComplete: 10, NumIncomplete: 40},
	}}
	s, dir := newTestSync(t, fake)
	s.Exclude = ExcludeRules{MaxSeeders: 100}

	s.poll(context.Background())

	if fake.exported["h1"] != 0 {
		t.Error("excluded torrent h1 was exported")
	}
	if _, err := os.Stat(filepath.Join(dir, "h1.torrent")); !os.IsNotExist(err) {
		t.Error("excluded torrent h1 file exists in torrents dir")
	}
	if fake.exported["h2"] != 1 {
		t.Error("eligible torrent h2 was not exported")
	}
	st := s.Status()
	if len(st.Excluded) != 1 || st.Excluded[0].Hash != "h1" || st.Excluded[0].Reason == "" {
		t.Errorf("status excluded = %+v, want h1 with a reason", st.Excluded)
	}
	if st.Tracked != 1 {
		t.Errorf("tracked = %d, want 1", st.Tracked)
	}

	// Swarm thins out: h1 becomes eligible and must be imported on the
	// next poll (excluded torrents are never marked seen).
	fake.torrents[0].NumComplete = 50
	s.poll(context.Background())
	if fake.exported["h1"] != 1 {
		t.Error("h1 was not imported after becoming eligible")
	}
	if len(s.Status().Excluded) != 0 {
		t.Errorf("excluded should be empty now, got %+v", s.Status().Excluded)
	}
}

func TestSyncEvictsWhenSwarmCrossesThreshold(t *testing.T) {
	fake := &fakeQBittorrent{exported: map[string]int{}, torrents: []TorrentInfo{
		{Hash: "h1", Name: "Borderline", Tracker: "https://tracker.example.org/announce", NumComplete: 10, NumIncomplete: 40},
	}}
	s, dir := newTestSync(t, fake)
	s.Exclude = ExcludeRules{MaxSeeders: 100}
	s.Evict = true
	var evicted []string
	s.EvictFn = func(hash string) { evicted = append(evicted, hash) }

	s.poll(context.Background())
	path := filepath.Join(dir, "h1.torrent")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("precondition: torrent file should exist: %v", err)
	}

	fake.torrents[0].NumComplete = 5000
	s.poll(context.Background())

	if len(evicted) != 1 || evicted[0] != "h1" {
		t.Errorf("evicted = %v, want [h1]", evicted)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("evicted torrent file still exists")
	}
	if s.Status().Tracked != 0 {
		t.Errorf("tracked = %d, want 0 after eviction", s.Status().Tracked)
	}
}

func TestSyncEvictionOffByDefault(t *testing.T) {
	fake := &fakeQBittorrent{exported: map[string]int{}, torrents: []TorrentInfo{
		{Hash: "h1", Name: "Borderline", Tracker: "https://tracker.example.org/announce", NumComplete: 10, NumIncomplete: 40},
	}}
	s, dir := newTestSync(t, fake)
	s.Exclude = ExcludeRules{MaxSeeders: 100}
	// Evict deliberately left false.

	s.poll(context.Background())
	fake.torrents[0].NumComplete = 5000
	s.poll(context.Background())

	if _, err := os.Stat(filepath.Join(dir, "h1.torrent")); err != nil {
		t.Error("without evict, already-imported torrent file must stay")
	}
	if s.Status().Tracked != 1 {
		t.Errorf("tracked = %d, want 1 (eviction off)", s.Status().Tracked)
	}
}
