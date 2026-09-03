// Package watcher polls a directory for .torrent files, the same low-tech
// approach Joal uses (a filesystem watch API is overkill for a directory
// that changes a few times an hour, and polling works identically across
// every filesystem ratioreign might run on in a container).
package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/torrentfile"
)

type Watcher struct {
	Dir      string
	Interval time.Duration
	OnAdd    func(*torrentfile.Torrent)

	seen map[string]bool
}

func New(dir string, interval time.Duration, onAdd func(*torrentfile.Torrent)) *Watcher {
	return &Watcher{Dir: dir, Interval: interval, OnAdd: onAdd, seen: make(map[string]bool)}
}

// Run scans immediately, then every Interval, until stop is closed.
func (w *Watcher) Run(stop <-chan struct{}) {
	w.scan()
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			w.scan()
		case <-stop:
			return
		}
	}
}

func (w *Watcher) scan() {
	entries, err := os.ReadDir(w.Dir)
	if err != nil {
		slog.Warn("watcher: cannot read torrents dir", "dir", w.Dir, "error", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".torrent") {
			continue
		}
		path := filepath.Join(w.Dir, e.Name())
		if w.seen[path] {
			continue
		}
		w.seen[path] = true
		t, err := torrentfile.Load(path)
		if err != nil {
			slog.Warn("watcher: failed to parse torrent, skipping", "file", path, "error", err)
			continue
		}
		w.OnAdd(t)
	}
}
