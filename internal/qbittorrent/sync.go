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

	// Exclude filters which qBittorrent torrents get imported. Excluded
	// torrents are never marked seen — swarm counts change, so one that's
	// excluded now (e.g. too many seeders) is imported later if it becomes
	// eligible.
	Exclude ExcludeRules
	// Evict, when true, also applies Exclude to torrents already imported:
	// EvictFn (wired by main to the seeder manager) stops the fake seed,
	// and the exported .torrent file is removed so it isn't re-imported.
	Evict   bool
	EvictFn func(hash string)

	seen map[string]bool

	mu         sync.Mutex
	lastPollAt time.Time
	lastError  string
	trackedN   int
	excluded   []ExcludedTorrent
}

// ExcludedTorrent is a torrent currently kept out by the Exclude rules,
// with the reason — surfaced in Status so "why didn't it sync?" is
// answerable from the UI instead of only from container logs.
type ExcludedTorrent struct {
	Hash   string `json:"hash"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

func NewSync(client *Client, dir, category string, pollInterval time.Duration) *Sync {
	return &Sync{Client: client, Dir: dir, Category: category, PollInterval: pollInterval, seen: make(map[string]bool)}
}

// Status is a snapshot of the sync loop's health, exposed over the API so a
// broken connection (wrong URL, blocked by a firewall, rejected by
// qBittorrent) is visible from the web UI instead of only in container logs.
type Status struct {
	LastPollAt time.Time         `json:"lastPollAt"`
	LastError  string            `json:"lastError,omitempty"`
	Tracked    int               `json:"torrentsTracked"`
	Excluded   []ExcludedTorrent `json:"excluded,omitempty"`
}

func (s *Sync) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Status{LastPollAt: s.lastPollAt, LastError: s.lastError, Tracked: s.trackedN, Excluded: s.excluded}
}

func (s *Sync) setResult(err error) {
	s.setResultExcluded(err, nil)
}

func (s *Sync) setResultExcluded(err error, excluded []ExcludedTorrent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPollAt = time.Now()
	s.trackedN = len(s.seen)
	s.excluded = excluded
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

	var excludedList []ExcludedTorrent
	for _, info := range torrents {
		path := filepath.Join(s.Dir, info.Hash+".torrent")
		if s.Exclude.Active() {
			if excluded, reason := Evaluate(info, s.trackerURLs(ctx, info), s.Exclude); excluded {
				excludedList = append(excludedList, ExcludedTorrent{Hash: info.Hash, Name: info.Name, Reason: reason})
				_, fileErr := os.Stat(path)
				if s.Evict && (s.seen[info.Hash] || fileErr == nil) {
					if s.EvictFn != nil {
						s.EvictFn(info.Hash)
					}
					if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
						slog.Warn("qbittorrent: failed to remove evicted torrent file", "path", path, "error", rmErr)
					}
					delete(s.seen, info.Hash)
					slog.Info("qbittorrent: evicted excluded torrent", "name", info.Name, "hash", info.Hash, "reason", reason)
				}
				continue
			}
		}
		if s.seen[info.Hash] {
			continue
		}
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

	s.setResultExcluded(nil, excludedList)
}

// trackerURLs returns every tracker URL to test the tracker rules against:
// the full torrents/trackers list when tracker rules are configured
// (torrents/info names only the active tracker), falling back to that one
// URL when the extra call isn't needed or fails.
func (s *Sync) trackerURLs(ctx context.Context, info TorrentInfo) []string {
	if len(s.Exclude.Trackers) == 0 {
		return []string{info.Tracker}
	}
	entries, err := s.Client.ListTrackers(ctx, info.Hash)
	if err != nil {
		slog.Warn("qbittorrent: failed to list trackers, matching active tracker only", "hash", info.Hash, "error", err)
		return []string{info.Tracker}
	}
	urls := make([]string, 0, len(entries))
	for _, e := range entries {
		urls = append(urls, e.URL)
	}
	return urls
}
