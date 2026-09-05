package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
)

// rssFeedView and qbittorrentView mirror config.RSSFeed / QBittorrentConfig
// but with time.Duration rendered as a human string ("10m") instead of
// nanoseconds, since that's what a plain HTML form field can hold.
type rssFeedView struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	PollInterval string `json:"pollInterval"`
}

type qbittorrentView struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`
	Username string `json:"username"`
	// Password is never populated on GET — only PasswordSet is, so the UI
	// can show "a password is saved" without ever round-tripping the
	// secret to the browser. On PUT, a blank Password means "leave it
	// unchanged"; there's no way to explicitly clear it, but disabling
	// qBittorrent sync entirely covers that case.
	Password     string `json:"password"`
	PasswordSet  bool   `json:"passwordSet"`
	PollInterval string `json:"pollInterval"`
	Category     string `json:"category"`
}

type configView struct {
	ListenAddr                  string           `json:"listenAddr"`
	MinUploadRateKBs            int              `json:"minUploadRateKBs"`
	MaxUploadRateKBs            int              `json:"maxUploadRateKBs"`
	SimultaneousSeed            int              `json:"simultaneousSeed"`
	ClientProfile               string           `json:"clientProfile"`
	KeepTorrentWithZeroLeechers bool             `json:"keepTorrentWithZeroLeechers"`
	UploadRatioTarget           float64          `json:"uploadRatioTarget"`
	TorrentsDir                 string           `json:"torrentsDir"`
	ArchiveDir                  string           `json:"archiveDir"`
	ProfilesDir                 string           `json:"profilesDir"`
	RSS                         []rssFeedView    `json:"rss"`
	QBittorrent                 *qbittorrentView `json:"qbittorrent"`
}

func toConfigView(c *config.Config) configView {
	v := configView{
		ListenAddr:                  c.ListenAddr,
		MinUploadRateKBs:            c.MinUploadRateKBs,
		MaxUploadRateKBs:            c.MaxUploadRateKBs,
		SimultaneousSeed:            c.SimultaneousSeed,
		ClientProfile:               c.ClientProfile,
		KeepTorrentWithZeroLeechers: c.KeepTorrentWithZeroLeechers,
		UploadRatioTarget:           c.UploadRatioTarget,
		TorrentsDir:                 c.TorrentsDir,
		ArchiveDir:                  c.ArchiveDir,
		ProfilesDir:                 c.ProfilesDir,
		RSS:                         []rssFeedView{},
	}
	for _, f := range c.RSS {
		v.RSS = append(v.RSS, rssFeedView{Name: f.Name, URL: f.URL, PollInterval: f.PollInterval.String()})
	}
	if c.QBittorrent != nil {
		v.QBittorrent = &qbittorrentView{
			Enabled:      c.QBittorrent.Enabled,
			URL:          c.QBittorrent.URL,
			Username:     c.QBittorrent.Username,
			PasswordSet:  c.QBittorrent.Password != "",
			PollInterval: c.QBittorrent.PollInterval.String(),
			Category:     c.QBittorrent.Category,
		}
	}
	return v
}

// fromConfigView builds a new *config.Config from a submitted view. existing
// (the config currently running) supplies the qBittorrent password when the
// view leaves it blank.
func fromConfigView(v configView, existing *config.Config) (*config.Config, error) {
	c := &config.Config{
		ListenAddr:                  v.ListenAddr,
		MinUploadRateKBs:            v.MinUploadRateKBs,
		MaxUploadRateKBs:            v.MaxUploadRateKBs,
		SimultaneousSeed:            v.SimultaneousSeed,
		ClientProfile:               v.ClientProfile,
		KeepTorrentWithZeroLeechers: v.KeepTorrentWithZeroLeechers,
		UploadRatioTarget:           v.UploadRatioTarget,
		TorrentsDir:                 v.TorrentsDir,
		ArchiveDir:                  v.ArchiveDir,
		ProfilesDir:                 v.ProfilesDir,
	}
	for i, f := range v.RSS {
		d, err := parseOptionalDuration(f.PollInterval)
		if err != nil {
			return nil, fmt.Errorf("rss[%d]: invalid pollInterval %q: %w", i, f.PollInterval, err)
		}
		c.RSS = append(c.RSS, config.RSSFeed{Name: f.Name, URL: f.URL, PollInterval: d})
	}
	if v.QBittorrent != nil {
		d, err := parseOptionalDuration(v.QBittorrent.PollInterval)
		if err != nil {
			return nil, fmt.Errorf("qbittorrent: invalid pollInterval %q: %w", v.QBittorrent.PollInterval, err)
		}
		password := v.QBittorrent.Password
		if password == "" && existing != nil && existing.QBittorrent != nil {
			password = existing.QBittorrent.Password
		}
		c.QBittorrent = &config.QBittorrentConfig{
			Enabled:      v.QBittorrent.Enabled,
			URL:          v.QBittorrent.URL,
			Username:     v.QBittorrent.Username,
			Password:     password,
			PollInterval: d,
			Category:     v.QBittorrent.Category,
		}
	}
	return c, nil
}

func parseOptionalDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toConfigView(s.Config))
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var v configView
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	cfg, err := fromConfigView(v, s.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := cfg.Save(s.ConfigPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "config saved; restarting to apply changes"})
	if s.RequestRestart != nil {
		s.RequestRestart()
	}
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.Config.ProfilesDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".client") {
			names = append(names, strings.TrimSuffix(e.Name(), ".client"))
		}
	}
	writeJSON(w, http.StatusOK, names)
}
