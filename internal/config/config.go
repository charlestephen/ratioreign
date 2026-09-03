// Package config loads ratioreign's YAML configuration file.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// ListenAddr is the address the status/control REST API binds to.
	ListenAddr string `yaml:"listenAddr"`

	// MinUploadRateKBs/MaxUploadRateKBs bound the fake aggregate upload
	// speed (kB/s) reported across all currently-seeding torrents.
	MinUploadRateKBs int `yaml:"minUploadRateKBs"`
	MaxUploadRateKBs int `yaml:"maxUploadRateKBs"`

	// SimultaneousSeed caps how many torrents announce as "seeding" at once.
	SimultaneousSeed int `yaml:"simultaneousSeed"`

	// ClientProfile names a *.client file (without extension) in ProfilesDir
	// to impersonate.
	ClientProfile string `yaml:"clientProfile"`

	// KeepTorrentWithZeroLeechers: if false, a torrent whose tracker reports
	// zero leechers is archived (there's no ratio to farm from real demand).
	KeepTorrentWithZeroLeechers bool `yaml:"keepTorrentWithZeroLeechers"`

	// UploadRatioTarget archives a torrent once its session
	// uploaded-bytes/size ratio reaches this value. -1 disables the check.
	UploadRatioTarget float64 `yaml:"uploadRatioTarget"`

	// TorrentsDir is watched for .torrent files to seed; ArchiveDir receives
	// torrents that are done (ratio target hit, zero leechers, or repeated
	// announce failures).
	TorrentsDir string `yaml:"torrentsDir"`
	ArchiveDir  string `yaml:"archiveDir"`
	ProfilesDir string `yaml:"profilesDir"`

	RSS         []RSSFeed          `yaml:"rss"`
	QBittorrent *QBittorrentConfig `yaml:"qbittorrent"`
}

type RSSFeed struct {
	Name         string        `yaml:"name"`
	URL          string        `yaml:"url"`
	PollInterval time.Duration `yaml:"pollInterval"`
}

type QBittorrentConfig struct {
	Enabled      bool          `yaml:"enabled"`
	URL          string        `yaml:"url"`
	Username     string        `yaml:"username"`
	Password     string        `yaml:"password"`
	PollInterval time.Duration `yaml:"pollInterval"`
	// Category, if set, only imports qBittorrent torrents in this category.
	Category string `yaml:"category"`
}

func defaults() Config {
	return Config{
		ListenAddr:                  ":7070",
		MinUploadRateKBs:            30,
		MaxUploadRateKBs:            160,
		SimultaneousSeed:            20,
		KeepTorrentWithZeroLeechers: true,
		UploadRatioTarget:           -1,
		TorrentsDir:                 "./data/torrents",
		ArchiveDir:                  "./data/torrents/archived",
		ProfilesDir:                 "./profiles",
	}
}

// Load reads and validates the config file at path, filling in defaults for
// unset fields.
func Load(path string) (*Config, error) {
	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.MinUploadRateKBs < 0 {
		return fmt.Errorf("minUploadRateKBs must be >= 0")
	}
	if c.MaxUploadRateKBs < c.MinUploadRateKBs {
		return fmt.Errorf("maxUploadRateKBs (%d) must be >= minUploadRateKBs (%d)", c.MaxUploadRateKBs, c.MinUploadRateKBs)
	}
	if c.SimultaneousSeed < 1 {
		return fmt.Errorf("simultaneousSeed must be >= 1")
	}
	if c.ClientProfile == "" {
		return fmt.Errorf("clientProfile is required")
	}
	for i, feed := range c.RSS {
		if feed.URL == "" {
			return fmt.Errorf("rss[%d]: url is required", i)
		}
		if feed.PollInterval <= 0 {
			c.RSS[i].PollInterval = 10 * time.Minute
		}
	}
	if c.QBittorrent != nil && c.QBittorrent.Enabled {
		if c.QBittorrent.URL == "" {
			return fmt.Errorf("qbittorrent.url is required when qbittorrent.enabled is true")
		}
		if c.QBittorrent.PollInterval <= 0 {
			c.QBittorrent.PollInterval = 30 * time.Second
		}
	}
	return nil
}
