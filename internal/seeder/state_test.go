package seeder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := map[string]int64{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 123456789,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": 0,
	}
	if err := saveState(path, want); err != nil {
		t.Fatal(err)
	}
	got := loadState(path)
	if len(got) != len(want) {
		t.Fatalf("loadState returned %d entries, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("loadState[%q] = %d, want %d", k, got[k], v)
		}
	}
}

func TestLoadStateMissingFileReturnsEmptyMap(t *testing.T) {
	got := loadState(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if got == nil {
		t.Fatal("loadState should return an empty map, not nil, for a missing file")
	}
	if len(got) != 0 {
		t.Fatalf("loadState of a missing file returned %d entries, want 0", len(got))
	}
}

func TestLoadStateCorruptFileReturnsEmptyMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadState(path)
	if len(got) != 0 {
		t.Fatalf("loadState of a corrupt file returned %d entries, want 0 (fail open, not crash)", len(got))
	}
}
