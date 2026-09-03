// Package seeder is ratioreign's core: it owns the set of torrents currently
// announcing as "seeding" to their trackers, drives the announce schedule
// for each, and fakes their uploaded-byte counters. It never opens a peer
// connection or transfers a single byte of payload data — the entire
// "seed" is the tracker believing one exists.
package seeder

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/announce"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/profile"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/speed"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/torrentfile"
)

// ArchiveReason explains why a torrent stopped being seeded.
type ArchiveReason string

const (
	ArchiveManual       ArchiveReason = "manual"
	ArchiveZeroLeechers ArchiveReason = "zero_leechers"
	ArchiveRatioReached ArchiveReason = "ratio_target_reached"
	ArchiveTrackerDead  ArchiveReason = "too_many_failed_announces"
)

const maxConsecutiveFails = 5

// Torrent is the live state of one torrent being seeded or waiting for a
// free slot.
type Torrent struct {
	File    *torrentfile.Torrent
	session *profile.Session

	Uploaded         int64
	Seeders          int
	Leechers         int
	trackerIdx       int
	consecutiveFails int
	nextAnnounceAt   time.Time
	AddedAt          time.Time
	LastAnnounceAt   time.Time
	LastError        string
	Active           bool // true once it has an announce slot (vs. waiting in the pending queue)
	port             int
}

// Status is the read-only snapshot exposed to the API.
type Status struct {
	InfoHash string    `json:"infoHash"`
	Name     string    `json:"name"`
	Uploaded int64     `json:"uploaded"`
	Seeders  int       `json:"seeders"`
	Leechers int       `json:"leechers"`
	Active   bool      `json:"active"`
	AddedAt  time.Time `json:"addedAt"`
	LastErr  string    `json:"lastError,omitempty"`
	Ratio    float64   `json:"ratio"`
}

// Manager coordinates every seeded torrent.
type Manager struct {
	cfg       *config.Config
	profile   *profile.Profile
	client    *announce.Client
	speed     *speed.Provider
	onArchive func(t *Torrent, reason ArchiveReason)

	mu      sync.Mutex
	items   map[string]*Torrent // keyed by info hash hex
	running bool
}

func NewManager(cfg *config.Config, prof *profile.Profile) *Manager {
	return &Manager{
		cfg:     cfg,
		profile: prof,
		client:  announce.NewClient(prof),
		speed:   speed.NewProvider(cfg.MinUploadRateKBs, cfg.MaxUploadRateKBs, 20*time.Minute),
		items:   make(map[string]*Torrent),
	}
}

// OnArchive registers a callback invoked whenever a torrent is archived, so
// the watched-folder layer can move its .torrent file into ArchiveDir.
func (m *Manager) OnArchive(f func(t *Torrent, reason ArchiveReason)) { m.onArchive = f }

// Add registers a new torrent. It starts seeding immediately if a slot is
// free, otherwise waits in the pending queue.
func (m *Manager) Add(t *torrentfile.Torrent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := t.InfoHashHex()
	if _, exists := m.items[key]; exists {
		return
	}
	m.items[key] = &Torrent{File: t, AddedAt: time.Now(), port: randomPort()}
	m.fillSlotsLocked()
}

// Remove stops seeding a torrent immediately (best-effort stopped announce)
// and drops it.
func (m *Manager) Remove(infoHash string) {
	m.mu.Lock()
	item, ok := m.items[infoHash]
	if ok {
		delete(m.items, infoHash)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	if item.Active {
		m.sendAnnounce(context.Background(), item, "stopped")
	}
	m.fillSlots()
}

// Snapshot returns a stable-ordered list of all known torrents.
func (m *Manager) Snapshot() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.items))
	for hash, t := range m.items {
		ratio := 0.0
		if t.File.TotalSize > 0 {
			ratio = float64(t.Uploaded) / float64(t.File.TotalSize)
		}
		out = append(out, Status{
			InfoHash: hash,
			Name:     t.File.Name,
			Uploaded: t.Uploaded,
			Seeders:  t.Seeders,
			Leechers: t.Leechers,
			Active:   t.Active,
			AddedAt:  t.AddedAt,
			LastErr:  t.LastError,
			Ratio:    ratio,
		})
	}
	return out
}

// Run drives the bandwidth and announce-scheduling loop until stop is
// closed.
func (m *Manager) Run(stop <-chan struct{}) {
	go m.speed.Run(stop)

	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			m.tick()
		case <-stop:
			m.stopAll()
			return
		}
	}
}

func (m *Manager) tick() {
	now := time.Now()

	m.mu.Lock()
	var stats []speed.Stats
	var due []*Torrent
	for hash, t := range m.items {
		if !t.Active {
			continue
		}
		stats = append(stats, speed.Stats{Key: hash, Seeders: t.Seeders, Leechers: t.Leechers})
		if !t.nextAnnounceAt.IsZero() && now.After(t.nextAnnounceAt) {
			due = append(due, t)
		}
	}
	rates := speed.Distribute(m.speed.Current(), stats)
	for hash, bps := range rates {
		m.items[hash].Uploaded += bps
	}
	m.mu.Unlock()

	for _, t := range due {
		go m.sendAnnounce(context.Background(), t, "")
	}
}

func (m *Manager) fillSlots() {
	m.mu.Lock()
	m.fillSlotsLocked()
	m.mu.Unlock()
}

func (m *Manager) fillSlotsLocked() {
	activeCount := 0
	for _, t := range m.items {
		if t.Active {
			activeCount++
		}
	}
	if activeCount >= m.cfg.SimultaneousSeed {
		return
	}
	for _, t := range m.items {
		if activeCount >= m.cfg.SimultaneousSeed {
			return
		}
		if t.Active {
			continue
		}
		t.Active = true
		activeCount++
		go m.sendAnnounce(context.Background(), t, "started")
	}
}

func (m *Manager) sendAnnounce(ctx context.Context, t *Torrent, event string) {
	m.mu.Lock()
	if t.session == nil {
		session, err := m.profile.NewSession()
		if err != nil {
			t.LastError = err.Error()
			m.mu.Unlock()
			return
		}
		t.session = session
	}
	trackerURL := t.File.Announce[t.trackerIdx%len(t.File.Announce)]
	session := t.session
	uploaded := t.Uploaded
	left := t.File.TotalSize - t.Uploaded // we never actually hold data; left mirrors "still farming" for trackers that gate ratio on it
	if left < 0 {
		left = 0
	}
	port := t.port
	m.mu.Unlock()

	params := profile.QueryParams{
		InfoHash: t.File.InfoHash,
		Port:     port,
		Uploaded: uploaded,
		Left:     left,
		Event:    event,
	}

	resp, err := m.client.Announce(ctx, session, trackerURL, params)

	m.mu.Lock()
	defer m.mu.Unlock()
	// The torrent may have been removed while the request was in flight.
	if _, ok := m.items[t.File.InfoHashHex()]; !ok {
		return
	}

	if err != nil || resp.FailureReason != "" {
		t.consecutiveFails++
		if err != nil {
			t.LastError = err.Error()
			slog.Warn("announce failed", "torrent", t.File.Name, "tracker", trackerURL, "error", err)
		} else {
			t.LastError = resp.FailureReason
			slog.Warn("tracker rejected announce", "torrent", t.File.Name, "tracker", trackerURL, "reason", resp.FailureReason)
		}
		t.trackerIdx++
		if t.consecutiveFails >= maxConsecutiveFails {
			m.archiveLocked(t, ArchiveTrackerDead)
			return
		}
		t.nextAnnounceAt = time.Now().Add(30 * time.Second)
		return
	}

	t.consecutiveFails = 0
	t.LastError = ""
	t.LastAnnounceAt = time.Now()
	t.Seeders = resp.Complete
	t.Leechers = resp.Incomplete
	interval := resp.Interval
	if interval <= 0 {
		interval = 300
	}
	// Jitter the next announce slightly so many torrents on the same
	// tracker don't all fire in lockstep.
	jitter := time.Duration(rand.Intn(5)) * time.Second
	t.nextAnnounceAt = time.Now().Add(time.Duration(interval)*time.Second + jitter)

	if event == "stopped" {
		return
	}

	if !m.cfg.KeepTorrentWithZeroLeechers && t.Seeders+t.Leechers > 0 && t.Leechers == 0 {
		m.archiveLocked(t, ArchiveZeroLeechers)
		return
	}
	if m.cfg.UploadRatioTarget > 0 && t.File.TotalSize > 0 {
		ratio := float64(t.Uploaded) / float64(t.File.TotalSize)
		if ratio >= m.cfg.UploadRatioTarget {
			m.archiveLocked(t, ArchiveRatioReached)
			return
		}
	}
}

// archiveLocked removes a torrent while m.mu is held and notifies onArchive
// outside the lock via a goroutine to avoid deadlocking watched-folder
// callbacks that may call back into the Manager.
func (m *Manager) archiveLocked(t *Torrent, reason ArchiveReason) {
	delete(m.items, t.File.InfoHashHex())
	if m.onArchive != nil {
		go m.onArchive(t, reason)
	}
	go m.fillSlots()
}

func (m *Manager) stopAll() {
	m.mu.Lock()
	var active []*Torrent
	for _, t := range m.items {
		if t.Active {
			active = append(active, t)
		}
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, t := range active {
		wg.Add(1)
		go func(t *Torrent) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			m.sendAnnounce(ctx, t, "stopped")
		}(t)
	}
	wg.Wait()
}

func randomPort() int {
	return 6881 + rand.Intn(9)
}
