package qbittorrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionNormalizesLeadingV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/app/version" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("v5.1.2\n"))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != "5.1.2" {
		t.Errorf("Version = %q, want 5.1.2", v)
	}
}

func TestVersionAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Version(context.Background()); !IsNotAuthenticated(err) {
		t.Errorf("Version error = %v, want not-authenticated", err)
	}
}
