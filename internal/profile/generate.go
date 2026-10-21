package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GeneratedProfilesDir is where auto-generated *.client files live: next to
// the ratio state file, i.e. inside the writable data volume. profilesDir
// itself may be read-only (the published container image bakes profiles/
// in as root-owned files), so generated profiles can't go there.
func GeneratedProfilesDir(statePath string) string {
	return filepath.Join(filepath.Dir(statePath), "profiles")
}

// qbittorrentTemplate is the canonical qBittorrent fingerprint, identical
// to the shipped profiles/qbittorrent-*.client files. Across every bundled
// qBittorrent version the only version-dependent fields are the peer-id
// digits and the User-Agent — both substituted in GenerateQBittorrent.
func qbittorrentTemplate(peerDigits, version string) *Profile {
	return &Profile{
		KeyGenerator: &KeyGenerator{
			Algorithm: Algorithm{Type: "HASH_NO_LEADING_ZERO", Length: 8},
			RefreshOn: "TORRENT_PERSISTENT",
			KeyCase:   "upper",
		},
		PeerIDGenerator: PeerIDGenerator{
			Algorithm: Algorithm{
				Type:    "REGEX",
				Pattern: "-qB" + peerDigits + `-[A-Za-z0-9_~\(\)\!\.\*-]{12}`,
			},
			RefreshOn:       "NEVER",
			ShouldURLEncode: false,
		},
		URLEncoder: URLEncoder{
			EncodingExclusionPattern: `[A-Za-z0-9_~\(\)\!\.\*-]`,
			EncodedHexCase:           "lower",
		},
		Query:         "info_hash={infohash}&peer_id={peerid}&port={port}&uploaded={uploaded}&downloaded={downloaded}&left={left}&corrupt=0&key={key}&event={event}&numwant={numwant}&compact=1&no_peer_id=1&supportcrypto=1&redundant=0",
		Numwant:       200,
		NumwantOnStop: 0,
		RequestHeaders: []Header{
			{Name: "User-Agent", Value: "qBittorrent/" + version},
			{Name: "Accept-Encoding", Value: "gzip"},
			{Name: "Connection", Value: "close"},
		},
	}
}

// peerIDDigits encodes a version the way qBittorrent's peer ids do:
// major, minor and patch concatenated as single digits plus a trailing 0
// (4.6.3 -> "4630", 5.2.3 -> "5230"), matching every shipped profile.
func peerIDDigits(major, minor, patch int) string {
	return fmt.Sprintf("%d%d%d0", major, minor, patch)
}

// ParseVersion validates a bare "X.Y.Z" version string (no leading "v").
func ParseVersion(version string) (major, minor, patch int, err error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("profile: version %q is not X.Y.Z", version)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, convErr := strconv.Atoi(p)
		if convErr != nil || n < 0 {
			return 0, 0, 0, fmt.Errorf("profile: version %q has a non-numeric component", version)
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nil
}

// GenerateQBittorrent renders the .client JSON for a qBittorrent version.
func GenerateQBittorrent(version string) ([]byte, error) {
	major, minor, patch, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(qbittorrentTemplate(peerIDDigits(major, minor, patch), version), "", "    ")
	if err != nil {
		return nil, fmt.Errorf("profile: generate qbittorrent %s: %w", version, err)
	}
	return append(data, '\n'), nil
}

// EnsureQBittorrentProfile makes sure dir contains qbittorrent-<version>.client,
// generating it from the bundled template when missing. An existing file is
// never overwritten (it may be a hand-tuned community profile). Returns the
// profile name and whether the file was created.
func EnsureQBittorrentProfile(dir, version string) (string, bool, error) {
	if _, _, _, err := ParseVersion(version); err != nil {
		return "", false, err
	}
	name := "qbittorrent-" + version
	path := filepath.Join(dir, name+".client")
	if _, err := os.Stat(path); err == nil {
		return name, false, nil
	}
	data, err := GenerateQBittorrent(version)
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, fmt.Errorf("profile: create generated profiles dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", false, fmt.Errorf("profile: write generated profile: %w", err)
	}
	// Prove the generated file parses before anything tries to seed with it.
	if _, err := Load(path); err != nil {
		return "", false, fmt.Errorf("profile: generated profile is invalid: %w", err)
	}
	return name, true, nil
}
