package profile

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestPeerIDDigits(t *testing.T) {
	cases := map[string]string{
		"4.6.3": "4630",
		"5.2.3": "5230",
		"5.0.3": "5030",
		"4.3.9": "4390",
	}
	for version, want := range cases {
		maj, min, pat, err := ParseVersion(version)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", version, err)
		}
		if got := peerIDDigits(maj, min, pat); got != want {
			t.Errorf("peerIDDigits(%s) = %q, want %q", version, got, want)
		}
	}
}

func TestParseVersionRejectsBadInput(t *testing.T) {
	for _, v := range []string{"", "5.1", "v5.1.2", "a.b.c", "5.1.x", "5.1.2.1"} {
		if _, _, _, err := ParseVersion(v); err == nil {
			t.Errorf("ParseVersion(%q) should fail", v)
		}
	}
}

func TestGeneratedProfileMatchesShippedShape(t *testing.T) {
	dir := t.TempDir()
	name, created, err := EnsureQBittorrentProfile(dir, "5.1.2")
	if err != nil {
		t.Fatal(err)
	}
	if name != "qbittorrent-5.1.2" || !created {
		t.Fatalf("got name=%q created=%v, want qbittorrent-5.1.2/true", name, created)
	}

	p, err := Load(filepath.Join(dir, name+".client"))
	if err != nil {
		t.Fatal(err)
	}
	sess, err := p.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	peerPattern := regexp.MustCompile(`^-qB5120-[A-Za-z0-9_~\(\)\!\.\*-]{12}$`)
	q, err := sess.BuildQuery(QueryParams{Port: 6881, Left: 100})
	if err != nil {
		t.Fatal(err)
	}
	idPattern := regexp.MustCompile(`peer_id=([^&]+)`)
	m := idPattern.FindStringSubmatch(q)
	if m == nil || !peerPattern.MatchString(m[1]) {
		t.Errorf("generated peer id %q does not match %s (query %q)", m, peerPattern, q)
	}
	foundUA := false
	for _, h := range p.BuildHeaders() {
		if h.Name == "User-Agent" && h.Value == "qBittorrent/5.1.2" {
			foundUA = true
		}
	}
	if !foundUA {
		t.Error("generated profile lacks qBittorrent/5.1.2 User-Agent")
	}
}

func TestEnsureDoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qbittorrent-9.9.9.client")
	custom := []byte(`{"custom":true}`)
	if err := os.WriteFile(path, custom, 0o644); err != nil {
		t.Fatal(err)
	}
	_, created, err := EnsureQBittorrentProfile(dir, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("existing profile should not be regenerated")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(custom) {
		t.Error("existing profile was overwritten")
	}
}
