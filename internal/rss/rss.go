// Package rss polls torrent-site RSS feeds and drops newly-seen .torrent
// files into the watched torrents directory, where the watcher package picks
// them up like any manually-added torrent.
package rss

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type feedXML struct {
	Channel struct {
		Items []itemXML `xml:"item"`
	} `xml:"channel"`
}

type itemXML struct {
	Title     string `xml:"title"`
	Link      string `xml:"link"`
	GUID      string `xml:"guid"`
	Enclosure struct {
		URL  string `xml:"url,attr"`
		Type string `xml:"type,attr"`
	} `xml:"enclosure"`
}

// Feed describes one RSS source to poll.
type Feed struct {
	Name         string
	URL          string
	PollInterval time.Duration
}

// Watcher polls a set of RSS feeds and saves new .torrent files into Dir.
type Watcher struct {
	Feeds  []Feed
	Dir    string
	HTTP   *http.Client
	seenBy map[string]map[string]bool // feed name -> item key -> seen
}

func New(feeds []Feed, dir string) *Watcher {
	return &Watcher{
		Feeds:  feeds,
		Dir:    dir,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		seenBy: make(map[string]map[string]bool),
	}
}

// Run polls every feed on its own interval until stop is closed.
func (w *Watcher) Run(stop <-chan struct{}) {
	for _, f := range w.Feeds {
		go w.pollLoop(f, stop)
	}
}

func (w *Watcher) pollLoop(f Feed, stop <-chan struct{}) {
	w.poll(f)
	t := time.NewTicker(f.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			w.poll(f)
		case <-stop:
			return
		}
	}
}

func (w *Watcher) poll(f Feed) {
	seen, ok := w.seenBy[f.Name]
	if !ok {
		seen = make(map[string]bool)
		w.seenBy[f.Name] = seen
	}

	items, err := w.fetch(f.URL)
	if err != nil {
		slog.Warn("rss: fetch failed", "feed", f.Name, "error", err)
		return
	}

	for _, item := range items {
		key := item.GUID
		if key == "" {
			key = item.Enclosure.URL
		}
		if key == "" {
			key = item.Link
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true

		torrentURL := item.Enclosure.URL
		if torrentURL == "" {
			torrentURL = item.Link
		}
		if torrentURL == "" {
			continue
		}
		if err := w.download(f.Name, item.Title, torrentURL); err != nil {
			slog.Warn("rss: download failed", "feed", f.Name, "item", item.Title, "url", torrentURL, "error", err)
		}
	}
}

func (w *Watcher) fetch(feedURL string) ([]itemXML, error) {
	resp, err := w.HTTP.Get(feedURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	var feed feedXML
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	return feed.Channel.Items, nil
}

func (w *Watcher) download(feedName, title, torrentURL string) error {
	resp, err := w.HTTP.Get(torrentURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return err
	}
	// Filenames from feed titles aren't trustworthy path components; hash
	// the source URL instead so re-runs are idempotent and no traversal is
	// possible.
	sum := sha1.Sum([]byte(torrentURL))
	name := fmt.Sprintf("%s-%x.torrent", sanitize(feedName), sum[:8])
	path := filepath.Join(w.Dir, name)
	if _, err := os.Stat(path); err == nil {
		return nil // already downloaded
	}

	slog.Info("rss: downloaded torrent", "feed", feedName, "title", title, "file", name)
	return os.WriteFile(path, body, 0o644)
}

func sanitize(s string) string {
	b := []byte(s)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			b[i] = '_'
		}
	}
	return string(b)
}
