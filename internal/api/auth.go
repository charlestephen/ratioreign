package api

import (
	"crypto/subtle"
	"net/http"
)

// basicAuth wraps h with HTTP Basic Auth. /healthz is left open so
// container/orchestrator probes do not need credentials.
func basicAuth(username, password string, h http.Handler) http.Handler {
	expectedUser := []byte(username)
	expectedPass := []byte(password)
	realm := `Basic realm="RatioReign", charset="UTF-8"`

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			h.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), expectedUser) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), expectedPass) != 1 {
			w.Header().Set("WWW-Authenticate", realm)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}
