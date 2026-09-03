// Package torrentfile parses .torrent files into the fields ratioreign
// actually needs to fake an announce: the info_hash, the announce URLs, and
// a reported size (used only for the optional upload-ratio-target cutoff).
package torrentfile

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/bencode"
)

// Torrent holds the subset of a .torrent file's metadata relevant to
// announcing to a tracker without actually transferring data.
type Torrent struct {
	Path      string // source .torrent file path, used for watched-folder bookkeeping
	Name      string
	InfoHash  [20]byte
	Announce  []string // flattened, de-duplicated announce-list (falls back to single "announce")
	TotalSize int64    // sum of file lengths (or single "length" for a single-file torrent)
}

// Load reads and parses a .torrent file from disk.
func Load(path string) (*Torrent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
}

// Parse decodes raw .torrent file bytes. path is stored for bookkeeping only
// and may be empty.
func Parse(path string, data []byte) (*Torrent, error) {
	raw, _, err := bencode.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("torrentfile: %w", err)
	}
	dict, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("torrentfile: root value is not a dict")
	}

	infoRaw, err := bencode.RawInfoDict(data)
	if err != nil {
		return nil, fmt.Errorf("torrentfile: %w", err)
	}
	hash := sha1.Sum(infoRaw)

	info, _ := dict["info"].(map[string]any)
	name, _ := info["name"].([]byte)

	t := &Torrent{
		Path:     path,
		Name:     string(name),
		InfoHash: hash,
	}

	if length, ok := info["length"].(int64); ok {
		t.TotalSize = length
	} else if files, ok := info["files"].([]any); ok {
		for _, f := range files {
			fd, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if length, ok := fd["length"].(int64); ok {
				t.TotalSize += length
			}
		}
	}

	t.Announce = announceURLs(dict)
	if len(t.Announce) == 0 {
		return nil, errors.New("torrentfile: no announce URL found")
	}
	return t, nil
}

func announceURLs(dict map[string]any) []string {
	seen := make(map[string]bool)
	var urls []string
	add := func(v any) {
		b, ok := v.([]byte)
		if !ok || len(b) == 0 {
			return
		}
		s := string(b)
		if !seen[s] {
			seen[s] = true
			urls = append(urls, s)
		}
	}

	if tiers, ok := dict["announce-list"].([]any); ok {
		for _, tier := range tiers {
			list, ok := tier.([]any)
			if !ok {
				continue
			}
			for _, u := range list {
				add(u)
			}
		}
	}
	if announce, ok := dict["announce"]; ok {
		add(announce)
	}
	return urls
}

// InfoHashHex returns the lowercase hex representation of the info hash,
// used as the torrent's stable identifier in the API and archive bookkeeping.
func (t *Torrent) InfoHashHex() string {
	return fmt.Sprintf("%x", t.InfoHash[:])
}
