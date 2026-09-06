package qbittorrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientRejectsNonHTTPScheme(t *testing.T) {
	cases := []string{"ftp://qbt.example", "file:///etc/hosts", "not-a-url\x7f"}
	for _, u := range cases {
		if _, err := NewClient(u, "user", "pass"); err == nil {
			t.Errorf("NewClient(%q) should have been rejected, got nil error", u)
		}
	}
}

func TestNewClientAcceptsHTTPAndHTTPS(t *testing.T) {
	for _, u := range []string{"http://localhost:8080", "https://qbt.example:8443"} {
		if _, err := NewClient(u, "user", "pass"); err != nil {
			t.Errorf("NewClient(%q) should be accepted, got error: %v", u, err)
		}
	}
}

func TestLoginDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("redirect target should never be reached")
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	c, err := NewClient(redirector.URL, "user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background()); err == nil {
		t.Fatal("Login should fail when the server tries to redirect, not silently follow it")
	}
}
