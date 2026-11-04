package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
)

func TestBasicAuthProtectsAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /api/torrents", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})
	h := basicAuth("admin", "secret", mux)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/torrents", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /api/torrents: got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz should stay open: got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated request: got %d", rec.Code)
	}
}

func TestNewWrapsWithAuth(t *testing.T) {
	h := New(Deps{Config: &config.Config{Auth: config.AuthConfig{
		Enabled: true, Username: "u", Password: "p",
	}}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with auth enabled, got %d", rec.Code)
	}
}
