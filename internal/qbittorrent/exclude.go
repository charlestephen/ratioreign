package qbittorrent

import (
	"fmt"
	"net/url"
	"strings"
)

// ExcludeRules decides whether a qBittorrent torrent is kept out of
// ratioreign. All fields are optional; a zero rule set excludes nothing.
type ExcludeRules struct {
	// Trackers excludes a torrent when any of its tracker hosts contains
	// any of these substrings (case-insensitive), e.g. "example.org"
	// matches "https://tracker.example.org/announce".
	Trackers []string
	// Tags excludes a torrent carrying any of these tags (exact,
	// case-insensitive — qBittorrent tags are already discrete labels).
	Tags []string
	// MaxSeeders excludes a torrent whose swarm seeder count
	// (TorrentInfo.NumComplete) exceeds it. 0 disables the check.
	MaxSeeders int
	// MinLeechers excludes a torrent whose swarm leecher count
	// (TorrentInfo.NumIncomplete) is below it. 0 disables the check.
	MinLeechers int
}

// Active reports whether any rule is set.
func (r ExcludeRules) Active() bool {
	return len(r.Trackers) > 0 || len(r.Tags) > 0 || r.MaxSeeders > 0 || r.MinLeechers > 0
}

// Evaluate reports whether info is excluded by rules, with a human-readable
// reason naming the rule that fired. trackerURLs is every tracker URL known
// for the torrent (the active tracker from torrents/info, or the full list
// from torrents/trackers when tracker rules are configured).
func Evaluate(info TorrentInfo, trackerURLs []string, rules ExcludeRules) (bool, string) {
	for _, pattern := range rules.Trackers {
		p := strings.ToLower(strings.TrimSpace(pattern))
		if p == "" {
			continue
		}
		for _, raw := range trackerURLs {
			if strings.Contains(trackerHost(raw), p) {
				return true, fmt.Sprintf("tracker %s matches excluded %q", trackerHost(raw), pattern)
			}
		}
	}
	if len(rules.Tags) > 0 && info.Tags != "" {
		own := strings.Split(info.Tags, ",")
		for i := range own {
			own[i] = strings.ToLower(strings.TrimSpace(own[i]))
		}
		for _, pattern := range rules.Tags {
			p := strings.ToLower(strings.TrimSpace(pattern))
			for _, tag := range own {
				if p != "" && tag == p {
					return true, fmt.Sprintf("has excluded tag %q", pattern)
				}
			}
		}
	}
	if rules.MaxSeeders > 0 && info.NumComplete > rules.MaxSeeders {
		return true, fmt.Sprintf("seeders %d > maxSeeders %d", info.NumComplete, rules.MaxSeeders)
	}
	if rules.MinLeechers > 0 && info.NumIncomplete < rules.MinLeechers {
		return true, fmt.Sprintf("leechers %d < minLeechers %d", info.NumIncomplete, rules.MinLeechers)
	}
	return false, ""
}

// trackerHost extracts the lowercase host from a tracker URL. Entries that
// don't parse as URLs (qBittorrent's "[DHT]"/"[PeX]"/"[LSD]" pseudo-trackers)
// return "" and can never match a pattern.
func trackerHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
