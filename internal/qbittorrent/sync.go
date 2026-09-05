package qbittorrent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sync periodically mirrors qBittorrent's torrent list into a directory:
// any torrent added to qBittorrent gets its .torrent file pulled down and
// dropped there, where ratioreign's folder watcher will pick it up and
// start faking a seed for it too.
type Sync struct {
	Client       *Client
	Dir          string
	Category     string
	PollInterval time.Duration

	seen map[string]bool

	mu         sync.Mutex
	lastPollAt time.Time
	lastError  string
	trackedN   int
}

func NewSync(client *Client, dir, category string, pollInterval time.Duration) *Sync {
	return &Sync{Client: client, Dir: dir, Category: category, PollInterval: pollInterval, seen: make(map[string]bool)}
}

// Status is a snapshot of the sync loop's health, exposed over the API so a
// broken connection (wrong URL, blocked by a firewall, rejected by
// qBittorrent) is visible from the web UI instead of only in container logs.
type Status struct {
	LastPollAt time.Time `json:"lastPollAt"`
	LastError  string    `json:"lastError,omitempty"`
	Tracked    int       `json:"torrentsTracked"`
}

func (s *Sync) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Status{LastPollAt: s.lastPollAt, LastError: s.lastError, Tracked: s.trackedN}
}

func (s *Sync) setResult(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPollAt = time.Now()
	s.trackedN = len(s.seen)
	if err != nil {
		s.lastError = err.Error()
	} else {
		s.lastError = ""
	}
}

// Run logs in and polls until stop is closed. Login/poll errors are logged
// and retried on the next tick rather than treated as fatal — a temporarily
// unreachable qBittorrent instance shouldn't take down the rest of
// ratioreign.
func (s *Sync) Run(stop <-chan struct{}) {
	ctx := context.Background()
	if err := s.Client.Login(ctx); err != nil {
		slog.Warn("qbittorrent: initial login failed, will retry", "error", err)
	}

	s.poll(ctx)
	t := time.NewTicker(s.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.poll(ctx)
		case <-stop:
			return
		}
	}
}

func (s *Sync) poll(ctx context.Context) {
	torrents, err := s.Client.ListTorrents(ctx, s.Category)
	if IsNotAuthenticated(err) {
		if loginErr := s.Client.Login(ctx); loginErr != nil {
			slog.Warn("qbittorrent: re-login failed", "error", loginErr)
			s.setResult(loginErr)
			return
		}
		torrents, err = s.Client.ListTorrents(ctx, s.Category)
	}
	if err != nil {
		slog.Warn("qbittorrent: failed to list torrents", "error", err)
		s.setResult(err)
		return
	}

	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		slog.Warn("qbittorrent: cannot create torrents dir", "dir", s.Dir, "error", err)
		s.setResult(err)
		return
	}

	for _, info := range torrents {
		if s.seen[info.Hash] {
			continue
		}
		path := filepath.Join(s.Dir, info.Hash+".torrent")
		if _, statErr := os.Stat(path); statErr == nil {
			s.seen[info.Hash] = true
			continue
		}
		data, err := s.Client.ExportTorrent(ctx, info.Hash)
		if err != nil {
			slog.Warn("qbittorrent: failed to export torrent", "hash", info.Hash, "name", info.Name, "error", err)
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			slog.Warn("qbittorrent: failed to write torrent file", "path", path, "error", err)
			continue
		}
		slog.Info("qbittorrent: synced torrent", "name", info.Name, "hash", info.Hash)
		s.seen[info.Hash] = true
	}

	s.setResult(nil)
}
