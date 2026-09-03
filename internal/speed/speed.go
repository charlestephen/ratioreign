// Package speed simulates an aggregate upload rate and distributes it across
// active torrents by demand, exactly as Joal does: no bytes are ever
// transferred, this only decides how fast each torrent's reported "uploaded"
// counter climbs between announces.
package speed

import (
	"math/rand"
	"sync"
	"time"
)

// Stats is the minimal per-torrent demand signal used to weight bandwidth
// distribution: torrents with more leechers (and a higher leecher share of
// the swarm) get proportionally more of the fake upload rate, mirroring how
// a real client would naturally upload faster to swarms actively requesting
// data from it.
type Stats struct {
	Key      string
	Seeders  int
	Leechers int
}

func weight(s Stats) float64 {
	if s.Seeders == 0 && s.Leechers == 0 {
		return 0
	}
	total := float64(s.Seeders + s.Leechers)
	leecherRatio := float64(s.Leechers) / total
	return leecherRatio * leecherRatio * 100 * float64(s.Leechers)
}

// Provider periodically picks a new random aggregate speed within
// [min,max] kB/s and exposes it via Current.
type Provider struct {
	minBps int64
	maxBps int64
	period time.Duration

	mu      sync.RWMutex
	current int64
}

// NewProvider builds a Provider bounded by [minKBs,maxKBs] kB/s, re-rolling
// the speed every period.
func NewProvider(minKBs, maxKBs int, period time.Duration) *Provider {
	p := &Provider{minBps: int64(minKBs) * 1000, maxBps: int64(maxKBs) * 1000, period: period}
	p.reroll()
	return p
}

func (p *Provider) reroll() {
	var v int64
	if p.maxBps == p.minBps {
		v = p.minBps
	} else {
		v = p.minBps + rand.Int63n(p.maxBps-p.minBps)
	}
	p.mu.Lock()
	p.current = v
	p.mu.Unlock()
}

// Current returns the current aggregate speed in bytes/second.
func (p *Provider) Current() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.current
}

// Run re-rolls the speed every period until ctx-equivalent stop is closed.
func (p *Provider) Run(stop <-chan struct{}) {
	t := time.NewTicker(p.period)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			p.reroll()
		case <-stop:
			return
		}
	}
}

// Distribute splits totalBps across stats by weight, returning bytes/second
// for each torrent (keyed by Stats.Key). Torrents with zero weight (dead
// swarms) get a small floor share so they still accrue some upload rather
// than stalling outright, matching Joal's behaviour of never fully starving
// a torrent it's still announcing for.
func Distribute(totalBps int64, stats []Stats) map[string]int64 {
	out := make(map[string]int64, len(stats))
	if len(stats) == 0 {
		return out
	}
	const floorShare = 0.05
	weights := make([]float64, len(stats))
	var sum float64
	for i, s := range stats {
		w := weight(s)
		if w <= 0 {
			w = floorShare
		}
		weights[i] = w
		sum += w
	}
	for i, s := range stats {
		out[s.Key] = int64(float64(totalBps) * weights[i] / sum)
	}
	return out
}
