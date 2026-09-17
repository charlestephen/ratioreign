package seeder

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// persistedState is the on-disk shape of the ratio state file: cumulative
// uploaded bytes per torrent (keyed by info hash hex), surviving process
// restarts the same way qBittorrent's own resume data does. Entries are
// kept even after a torrent is archived (paused/done seeding) — only an
// explicit Remove() forgets a torrent's history, matching "delete" in a
// real client versus just pausing/stopping it.
type persistedState struct {
	Uploaded map[string]int64 `json:"uploaded"`
}

func loadState(path string) map[string]int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("seeder: failed to read state file, starting fresh", "path", path, "error", err)
		}
		return make(map[string]int64)
	}
	var s persistedState
	if err := json.Unmarshal(data, &s); err != nil {
		slog.Warn("seeder: state file is corrupt, starting fresh", "path", path, "error", err)
		return make(map[string]int64)
	}
	if s.Uploaded == nil {
		s.Uploaded = make(map[string]int64)
	}
	return s.Uploaded
}

// saveState writes state to path atomically (temp file + rename) so a crash
// mid-write never leaves a corrupt file for the next startup to trip over.
func saveState(path string, uploaded map[string]int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(persistedState{Uploaded: uploaded})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
