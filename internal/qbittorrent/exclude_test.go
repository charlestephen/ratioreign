package qbittorrent

import "testing"

func TestEvaluate(t *testing.T) {
	info := TorrentInfo{
		Hash: "abc", Name: "Some Torrent", Tracker: "https://tracker.example.org/announce?passkey=x",
		Tags: "Movies, keep", NumComplete: 120, NumIncomplete: 30,
	}
	active := []string{info.Tracker}

	cases := []struct {
		name    string
		rules   ExcludeRules
		urls    []string
		exclude bool
	}{
		{"no rules", ExcludeRules{}, active, false},
		{"tracker substring hit", ExcludeRules{Trackers: []string{"EXAMPLE.org"}}, active, true},
		{"tracker substring miss", ExcludeRules{Trackers: []string{"other.net"}}, active, false},
		{"tracker matched via secondary tracker", ExcludeRules{Trackers: []string{"backup.net"}},
			[]string{info.Tracker, "udp://backup.net:6969/announce"}, true},
		{"pseudo-tracker never matches", ExcludeRules{Trackers: []string{"dht"}},
			[]string{"[DHT]"}, false},
		{"tag hit case-insensitive", ExcludeRules{Tags: []string{"movies"}}, active, true},
		{"tag miss", ExcludeRules{Tags: []string{"tv"}}, active, false},
		{"seeders above max", ExcludeRules{MaxSeeders: 100}, active, true},
		{"seeders at max is fine", ExcludeRules{MaxSeeders: 120}, active, false},
		{"seeders check disabled at 0", ExcludeRules{MaxSeeders: 0}, active, false},
		{"leechers below min", ExcludeRules{MinLeechers: 31}, active, true},
		{"leechers at min is fine", ExcludeRules{MinLeechers: 30}, active, false},
	}
	for _, c := range cases {
		excluded, reason := Evaluate(info, c.urls, c.rules)
		if excluded != c.exclude {
			t.Errorf("%s: Evaluate = %v (reason %q), want %v", c.name, excluded, reason, c.exclude)
		}
		if excluded && reason == "" {
			t.Errorf("%s: excluded but no reason given", c.name)
		}
	}
}

func TestExcludeRulesActive(t *testing.T) {
	if (ExcludeRules{}).Active() {
		t.Error("zero rules should be inactive")
	}
	if !(ExcludeRules{MinLeechers: 1}).Active() {
		t.Error("minLeechers-only rules should be active")
	}
}
