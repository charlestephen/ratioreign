// Command ratioreign is a headless BitTorrent ratio-farming daemon: it
// announces to trackers as if it were seeding real torrents and reports a
// simulated upload rate, without ever transferring actual torrent data.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/api"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/profile"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/qbittorrent"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/rss"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/seeder"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/watcher"
)

func main() {
	configPath := flag.String("config", "./config/config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	profiles, err := profile.LoadDir(cfg.ProfilesDir)
	if err != nil {
		slog.Error("failed to load client profiles", "dir", cfg.ProfilesDir, "error", err)
		os.Exit(1)
	}
	activeProfile, ok := profiles[cfg.ClientProfile]
	if !ok {
		slog.Error("configured clientProfile not found", "clientProfile", cfg.ClientProfile, "profilesDir", cfg.ProfilesDir)
		os.Exit(1)
	}
	slog.Info("loaded client profile", "profile", activeProfile.Name)

	if err := os.MkdirAll(cfg.TorrentsDir, 0o755); err != nil {
		slog.Error("failed to create torrents dir", "dir", cfg.TorrentsDir, "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.ArchiveDir, 0o755); err != nil {
		slog.Error("failed to create archive dir", "dir", cfg.ArchiveDir, "error", err)
		os.Exit(1)
	}

	mgr := seeder.NewManager(cfg, activeProfile)
	mgr.OnArchive(func(t *seeder.Torrent, reason seeder.ArchiveReason) {
		slog.Info("torrent archived", "name", t.File.Name, "reason", reason)
		dest := filepath.Join(cfg.ArchiveDir, filepath.Base(t.File.Path))
		if err := os.Rename(t.File.Path, dest); err != nil {
			slog.Warn("failed to move archived torrent file", "path", t.File.Path, "error", err)
		}
	})

	stop := make(chan struct{})

	torrentWatcher := watcher.New(cfg.TorrentsDir, 5*time.Second, mgr.Add)
	go torrentWatcher.Run(stop)

	if len(cfg.RSS) > 0 {
		feeds := make([]rss.Feed, len(cfg.RSS))
		for i, f := range cfg.RSS {
			feeds[i] = rss.Feed{Name: f.Name, URL: f.URL, PollInterval: f.PollInterval}
		}
		rssWatcher := rss.New(feeds, cfg.TorrentsDir)
		go rssWatcher.Run(stop)
		slog.Info("rss feeds enabled", "count", len(feeds))
	}

	if cfg.QBittorrent != nil && cfg.QBittorrent.Enabled {
		qbClient, err := qbittorrent.NewClient(cfg.QBittorrent.URL, cfg.QBittorrent.Username, cfg.QBittorrent.Password)
		if err != nil {
			slog.Error("failed to create qbittorrent client", "error", err)
			os.Exit(1)
		}
		qbSync := qbittorrent.NewSync(qbClient, cfg.TorrentsDir, cfg.QBittorrent.Category, cfg.QBittorrent.PollInterval)
		go qbSync.Run(stop)
		slog.Info("qbittorrent sync enabled", "url", cfg.QBittorrent.URL)
	}

	go mgr.Run(stop)

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: api.New(mgr)}
	go func() {
		slog.Info("api listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("api server failed", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	slog.Info("shutting down")

	close(stop) // triggers stopped-announces for every active torrent

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	// Give in-flight "stopped" announces time to land before exiting.
	time.Sleep(2 * time.Second)
}
