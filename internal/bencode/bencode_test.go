package bencode

import (
	"reflect"
	"testing"
)

func TestDecodeRoundTrip(t *testing.T) {
	cases := map[string]any{
		"i42e":           int64(42),
		"i-7e":           int64(-7),
		"4:spam":         []byte("spam"),
		"l4:spam4:eggse": []any{[]byte("spam"), []byte("eggs")},
		"d3:cow3:moo4:spam4:eggse": map[string]any{
			"cow":  []byte("moo"),
			"spam": []byte("eggs"),
		},
	}
	for input, want := range cases {
		got, n, err := Decode([]byte(input))
		if err != nil {
			t.Fatalf("Decode(%q): %v", input, err)
		}
		if n != len(input) {
			t.Fatalf("Decode(%q): consumed %d bytes, want %d", input, n, len(input))
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Decode(%q) = %#v, want %#v", input, got, want)
		}
	}
}

func TestEncodeSortsKeys(t *testing.T) {
	v := map[string]any{"zebra": "z", "apple": "a"}
	got, err := Encode(v)
	if err != nil {
		t.Fatal(err)
	}
	want := "d5:apple1:a5:zebra1:ze"
	if string(got) != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}

func TestRawInfoDict(t *testing.T) {
	// A minimal single-file torrent: {announce, info: {name, length, piece length, pieces}}
	data := []byte("d8:announce18:http://tracker.io/4:infod6:lengthi10e4:name4:test12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	raw, err := RawInfoDict(data)
	if err != nil {
		t.Fatal(err)
	}
	want := "d6:lengthi10e4:name4:test12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaae"
	if string(raw) != want {
		t.Fatalf("RawInfoDict() = %q, want %q", raw, want)
	}
}
