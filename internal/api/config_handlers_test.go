package api

import (
	"testing"
	"time"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
)

func TestToConfigViewRedactsPassword(t *testing.T) {
	cfg := &config.Config{
		QBittorrent: &config.QBittorrentConfig{Enabled: true, Password: "supersecret", PollInterval: 30 * time.Second},
	}
	v := toConfigView(cfg)
	if v.QBittorrent.Password != "" {
		t.Fatalf("password leaked into view: %q", v.QBittorrent.Password)
	}
	if !v.QBittorrent.PasswordSet {
		t.Fatal("PasswordSet should be true when a password is configured")
	}
	if v.QBittorrent.PollInterval != "30s" {
		t.Fatalf("PollInterval = %q, want 30s", v.QBittorrent.PollInterval)
	}
}

func TestFromConfigViewBlankPasswordKeepsExisting(t *testing.T) {
	existing := &config.Config{
		QBittorrent: &config.QBittorrentConfig{Password: "original-secret"},
	}
	v := configView{
		ClientProfile:    "qbittorrent-4.6.3",
		SimultaneousSeed: 1,
		MaxUploadRateKBs: 10,
		QBittorrent: &qbittorrentView{
			Enabled: true, URL: "http://localhost:8080", Password: "", PollInterval: "30s",
		},
	}
	cfg, err := fromConfigView(v, existing)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.QBittorrent.Password != "original-secret" {
		t.Fatalf("password = %q, want original-secret to be preserved", cfg.QBittorrent.Password)
	}
}

func TestFromConfigViewNonBlankPasswordOverrides(t *testing.T) {
	existing := &config.Config{QBittorrent: &config.QBittorrentConfig{Password: "old"}}
	v := configView{
		ClientProfile:    "qbittorrent-4.6.3",
		SimultaneousSeed: 1,
		MaxUploadRateKBs: 10,
		QBittorrent:      &qbittorrentView{Enabled: true, URL: "http://localhost:8080", Password: "new-secret"},
	}
	cfg, err := fromConfigView(v, existing)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.QBittorrent.Password != "new-secret" {
		t.Fatalf("password = %q, want new-secret", cfg.QBittorrent.Password)
	}
}

func TestFromConfigViewInvalidDuration(t *testing.T) {
	v := configView{
		ClientProfile:    "qbittorrent-4.6.3",
		SimultaneousSeed: 1,
		MaxUploadRateKBs: 10,
		RSS:              []rssFeedView{{Name: "x", URL: "http://x", PollInterval: "not-a-duration"}},
	}
	if _, err := fromConfigView(v, nil); err == nil {
		t.Fatal("expected an error for an invalid pollInterval string")
	}
}
