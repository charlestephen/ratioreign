package torrentfile

import (
	"crypto/sha1"
	"testing"
)

func TestParse(t *testing.T) {
	data := []byte("d8:announce18:http://tracker.io/4:infod6:lengthi10e4:name4:test12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")

	tr, err := Parse("test.torrent", data)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Name != "test" {
		t.Errorf("Name = %q, want %q", tr.Name, "test")
	}
	if tr.TotalSize != 10 {
		t.Errorf("TotalSize = %d, want 10", tr.TotalSize)
	}
	if len(tr.Announce) != 1 || tr.Announce[0] != "http://tracker.io/" {
		t.Errorf("Announce = %v, want [http://tracker.io/]", tr.Announce)
	}

	wantInfo := "d6:lengthi10e4:name4:test12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaae"
	wantHash := sha1.Sum([]byte(wantInfo))
	if tr.InfoHash != wantHash {
		t.Errorf("InfoHash = %x, want %x", tr.InfoHash, wantHash)
	}
}

func TestParseAnnounceList(t *testing.T) {
	// announce-list takes priority and de-duplicates against the plain
	// "announce" field.
	data := []byte("d8:announce20:http://a.io/announce13:announce-listll20:http://a.io/announceel20:http://b.io/announceee4:infod6:lengthi1e4:name1:x12:piece lengthi1e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	tr, err := Parse("test.torrent", data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"http://a.io/announce", "http://b.io/announce"}
	if len(tr.Announce) != len(want) {
		t.Fatalf("Announce = %v, want %v", tr.Announce, want)
	}
	for i, u := range want {
		if tr.Announce[i] != u {
			t.Errorf("Announce[%d] = %q, want %q", i, tr.Announce[i], u)
		}
	}
}
