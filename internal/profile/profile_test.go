package profile

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

const testProfileJSON = `{
    "keyGenerator": {
        "algorithm": { "type": "HASH_NO_LEADING_ZERO", "length": 8 },
        "refreshOn": "TORRENT_PERSISTENT",
        "keyCase": "upper"
    },
    "peerIdGenerator": {
        "algorithm": { "type": "REGEX", "pattern": "-qB4630-[A-Za-z0-9_~\\(\\)\\!\\.\\*-]{12}" },
        "refreshOn": "NEVER",
        "shouldUrlEncode": false
    },
    "urlEncoder": {
        "encodingExclusionPattern": "[A-Za-z0-9_~\\(\\)\\!\\.\\*-]",
        "encodedHexCase": "lower"
    },
    "query": "info_hash={infohash}&peer_id={peerid}&port={port}&uploaded={uploaded}&downloaded={downloaded}&left={left}&key={key}&event={event}&numwant={numwant}",
    "numwant": 200,
    "numwantOnStop": 0,
    "requestHeaders": [
        { "name": "User-Agent", "value": "qBittorrent/4.6.3" }
    ]
}`

func loadTestProfile(t *testing.T) *Profile {
	t.Helper()
	var p Profile
	if err := json.Unmarshal([]byte(testProfileJSON), &p); err != nil {
		t.Fatal(err)
	}
	re, err := regexp.Compile(p.URLEncoder.EncodingExclusionPattern)
	if err != nil {
		t.Fatal(err)
	}
	p.exclusionRe = re
	p.Name = "test"
	return &p
}

func TestNewSessionPeerIDMatchesPattern(t *testing.T) {
	p := loadTestProfile(t)
	s, err := p.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^-qB4630-[A-Za-z0-9_~\(\)\!\.\*-]{12}$`)
	if !re.MatchString(s.peerID) {
		t.Errorf("peer id %q does not match expected pattern", s.peerID)
	}
	if len(s.key) != 8 {
		t.Errorf("key %q has length %d, want 8", s.key, len(s.key))
	}
	if s.key != strings.ToUpper(s.key) {
		t.Errorf("key %q should be uppercase", s.key)
	}
	if s.key[0] == '0' {
		t.Errorf("key %q should not have a leading zero", s.key)
	}
}

func TestBuildQueryDropsEmptyEvent(t *testing.T) {
	p := loadTestProfile(t)
	s, err := p.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.BuildQuery(QueryParams{Port: 6881, Uploaded: 100, Left: 900})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(q, "event=") {
		t.Errorf("query should not contain an empty event param: %q", q)
	}
	if !strings.Contains(q, "port=6881") {
		t.Errorf("query missing port: %q", q)
	}
	if !strings.Contains(q, "numwant=200") {
		t.Errorf("query missing numwant: %q", q)
	}
}

func TestBuildQueryKeepsStartedEvent(t *testing.T) {
	p := loadTestProfile(t)
	s, err := p.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.BuildQuery(QueryParams{Port: 6881, Event: "started"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "event=started") {
		t.Errorf("query missing event=started: %q", q)
	}
}

func TestURLEncodeBytesDoesNotMangleNonUTF8(t *testing.T) {
	p := loadTestProfile(t)
	// A 20-byte SHA1-like value containing bytes that are not valid UTF-8
	// on their own (e.g. 0xA3 as a lone byte). If urlEncode ranges over the
	// string as runes instead of bytes, invalid sequences silently become
	// U+FFFD (encoded as %EF%BF%BD) instead of the original byte.
	raw := []byte{0xa3, 0xe6, 0xc6, 0x01, 0x43, 0x69, 0x7a, 0x23, 0x36, 0x5a, 0x8e, 0x0e, 0x68, 0xda, 0x33, 0x93, 0x75, 0xa5, 0x00, 0xff}
	got := p.urlEncodeBytes(raw)
	if strings.Contains(got, "FFFD") || strings.Contains(got, "fffd") {
		t.Fatalf("urlEncodeBytes mangled non-UTF8 bytes into U+FFFD: %q", got)
	}
	want := "%a3%e6%c6%01Ciz%236Z%8e%0eh%da3%93u%a5%00%ff"
	if got != want {
		t.Fatalf("urlEncodeBytes(%x) = %q, want %q", raw, got, want)
	}
}

func TestRandomFromPatternHighByteClass(t *testing.T) {
	// rtorrent's real peer-id pattern is a Unicode code-point range,
	// U+0001 to U+00FF, which is what encoding/json decodes a *.client
	// file's pattern string into: a valid-UTF8 Go string of real runes,
	// never raw non-UTF8 bytes. randomFromPattern must still emit each
	// drawn value as a single raw output byte, not its multi-byte UTF-8
	// encoding, since a peer id is 20 raw bytes on the wire.
	pattern := "-lt0D60-[\u0001-\u00ff]{12}"
	s, err := randomFromPattern(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != len("-lt0D60-")+12 {
		t.Fatalf("randomFromPattern length = %d, want %d (UTF-8 encoding of high bytes would inflate this)", len(s), len("-lt0D60-")+12)
	}
}

func TestRandomFromPatternLength(t *testing.T) {
	s, err := randomFromPattern(`-TR4050-[A-Za-z0-9]{12}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != len("-TR4050-")+12 {
		t.Errorf("randomFromPattern length = %d, want %d", len(s), len("-TR4050-")+12)
	}
	if !strings.HasPrefix(s, "-TR4050-") {
		t.Errorf("randomFromPattern = %q, missing literal prefix", s)
	}
}
