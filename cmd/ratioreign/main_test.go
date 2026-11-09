package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/config"
)

// The generated admin password must never reach the log, in any auth state.
func TestLogAuthStatusNeverLogsPassword(t *testing.T) {
	const secret = "s3cr3t-pa55w0rd-value"
	for name, tc := range map[string]struct {
		enabled   bool
		generated bool
	}{
		"generated": {true, true},
		"enabled":   {true, false},
		"disabled":  {false, false},
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			cfg := &config.Config{Auth: config.AuthConfig{Enabled: tc.enabled, Username: "admin", Password: secret}}

			logAuthStatus(logger, cfg, "/etc/ratioreign/config.yaml", tc.generated)

			if buf.Len() == 0 {
				t.Fatal("expected a log line")
			}
			if strings.Contains(buf.String(), secret) {
				t.Fatalf("password leaked into log: %s", buf.String())
			}
			if tc.generated && !strings.Contains(buf.String(), "/etc/ratioreign/config.yaml") {
				t.Fatalf("generated-password line should name the config file: %s", buf.String())
			}
		})
	}
}
