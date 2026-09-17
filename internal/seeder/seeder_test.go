package seeder

import (
	"encoding/hex"
	"path/filepath"
	"testing"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/profile"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/torrentfile"
)

func TestAddResumesUploadedFromPersistedState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := saveState(statePath, map[string]int64{hash: 123456789}); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{MinUploadRateKBs: 10, MaxUploadRateKBs: 10, SimultaneousSeed: 1, StatePath: statePath}
	mgr := NewManager(cfg, &profile.Profile{})

	tor := &torrentfile.Torrent{Name: "test", Announce: []string{"http://tracker.example/announce"}, TotalSize: 1 << 30}
	raw, err := hex.DecodeString(hash)
	if err != nil {
		t.Fatal(err)
	}
	copy(tor.InfoHash[:], raw)

	mgr.Add(tor)

	snap := mgr.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("Snapshot() returned %d torrents, want 1", len(snap))
	}
	if snap[0].Uploaded != 123456789 {
		t.Errorf("Uploaded = %d, want 123456789 (resumed from persisted state, not reset to 0)", snap[0].Uploaded)
	}
}

func TestRemoveForgetsPersistedState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	cfg := &config.Config{MinUploadRateKBs: 10, MaxUploadRateKBs: 10, SimultaneousSeed: 1, StatePath: statePath}
	mgr := NewManager(cfg, &profile.Profile{})

	tor := &torrentfile.Torrent{Name: "test", Announce: []string{"http://tracker.example/announce"}, TotalSize: 1000}
	mgr.Add(tor)
	mgr.Remove(tor.InfoHashHex())

	got := loadState(statePath)
	if _, ok := got[tor.InfoHashHex()]; ok {
		t.Error("Remove should forget the torrent's persisted ratio history, but it's still in the state file")
	}
}
