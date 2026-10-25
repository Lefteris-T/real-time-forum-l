package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FORUM_ADDRESS", "")
	t.Setenv("FORUM_DATABASE_PATH", "")
	t.Setenv("FORUM_SESSION_DURATION", "")
	t.Setenv("FORUM_COOKIE_NAME", "")
	t.Setenv("FORUM_SECURE_COOKIE", "")
	t.Setenv("FORUM_HTTPS_ENABLED", "")
	t.Setenv("FORUM_TLS_CERT_FILE", "")
	t.Setenv("FORUM_TLS_KEY_FILE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Address != ":8080" {
		t.Errorf("Address = %q, want %q", cfg.Address, ":8080")
	}

	if cfg.DatabasePath != "data/realtime-forum.db" {
		t.Errorf(
			"DatabasePath = %q, want %q",
			cfg.DatabasePath,
			"data/realtime-forum.db",
		)
	}

	if cfg.SessionDuration != 24*time.Hour {
		t.Errorf(
			"SessionDuration = %v, want %v",
			cfg.SessionDuration,
			24*time.Hour,
		)
	}

	if cfg.CookieName != "forum_session" {
		t.Errorf(
			"CookieName = %q, want %q",
			cfg.CookieName,
			"forum_session",
		)
	}

	if cfg.SecureCookie {
		t.Error("SecureCookie = true, want false")
	}
	if cfg.HTTPSEnabled {
		t.Error("HTTPSEnabled = true, want false")
	}

	if cfg.TLSCertFile != "" {
		t.Errorf("TLSCertFile = %q, want empty", cfg.TLSCertFile)
	}

	if cfg.TLSKeyFile != "" {
		t.Errorf("TLSKeyFile = %q, want empty", cfg.TLSKeyFile)
	}
}
func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv("FORUM_ADDRESS", "127.0.0.1:9090")
	t.Setenv("FORUM_DATABASE_PATH", "testdata/forum.db")
	t.Setenv("FORUM_SESSION_DURATION", "2h")
	t.Setenv("FORUM_COOKIE_NAME", "custom_session")
	t.Setenv("FORUM_SECURE_COOKIE", "true")
	t.Setenv("FORUM_HTTPS_ENABLED", "true")
	t.Setenv("FORUM_TLS_CERT_FILE", "certs/localhost.crt")
	t.Setenv("FORUM_TLS_KEY_FILE", "certs/localhost.key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Address != "127.0.0.1:9090" {
		t.Errorf(
			"Address = %q, want %q",
			cfg.Address,
			"127.0.0.1:9090",
		)
	}

	if cfg.DatabasePath != "testdata/forum.db" {
		t.Errorf(
			"DatabasePath = %q, want %q",
			cfg.DatabasePath,
			"testdata/forum.db",
		)
	}

	if cfg.SessionDuration != 2*time.Hour {
		t.Errorf(
			"SessionDuration = %v, want %v",
			cfg.SessionDuration,
			2*time.Hour,
		)
	}

	if cfg.CookieName != "custom_session" {
		t.Errorf(
			"CookieName = %q, want %q",
			cfg.CookieName,
			"custom_session",
		)
	}

	if !cfg.SecureCookie {
		t.Error("SecureCookie = false, want true")
	}
	if !cfg.HTTPSEnabled {
		t.Error("HTTPSEnabled = false, want true")
	}

	if cfg.TLSCertFile != "certs/localhost.crt" {
		t.Errorf(
			"TLSCertFile = %q, want %q",
			cfg.TLSCertFile,
			"certs/localhost.crt",
		)
	}

	if cfg.TLSKeyFile != "certs/localhost.key" {
		t.Errorf(
			"TLSKeyFile = %q, want %q",
			cfg.TLSKeyFile,
			"certs/localhost.key",
		)
	}
}
func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
	}{
		{
			name:     "invalid address",
			envKey:   "FORUM_ADDRESS",
			envValue: "invalid-address",
		},
		{
			name:     "empty database path",
			envKey:   "FORUM_DATABASE_PATH",
			envValue: "   ",
		},
		{
			name:     "invalid session duration",
			envKey:   "FORUM_SESSION_DURATION",
			envValue: "tomorrow",
		},
		{
			name:     "zero session duration",
			envKey:   "FORUM_SESSION_DURATION",
			envValue: "0s",
		},
		{
			name:     "invalid cookie name",
			envKey:   "FORUM_COOKIE_NAME",
			envValue: "forum session",
		},
		{
			name:     "invalid secure cookie",
			envKey:   "FORUM_SECURE_COOKIE",
			envValue: "sometimes",
		},
		{
			name:     "invalid HTTPS enabled",
			envKey:   "FORUM_HTTPS_ENABLED",
			envValue: "sometimes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FORUM_ADDRESS", ":8080")
			t.Setenv("FORUM_DATABASE_PATH", "data/forum.db")
			t.Setenv("FORUM_SESSION_DURATION", "24h")
			t.Setenv("FORUM_COOKIE_NAME", "forum_session")
			t.Setenv("FORUM_SECURE_COOKIE", "false")

			t.Setenv("FORUM_HTTPS_ENABLED", "")
			t.Setenv("FORUM_TLS_CERT_FILE", "")
			t.Setenv("FORUM_TLS_KEY_FILE", "")
			t.Setenv(tt.envKey, tt.envValue)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want an error")
			}
		})
	}
}
func TestLoadValidatesHTTPSCertificateSettings(t *testing.T) {
	tests := []struct {
		name        string
		https       string
		certificate string
		key         string
		wantError   bool
	}{
		{
			name: "HTTPS disabled permits empty certificate settings",
		},
		{
			name:        "HTTPS enabled accepts certificate and key",
			https:       "true",
			certificate: "certs/localhost.crt",
			key:         "certs/localhost.key",
		},
		{
			name:        "HTTPS disabled accepts a complete pair",
			certificate: "certs/localhost.crt",
			key:         "certs/localhost.key",
		},
		{
			name:      "HTTPS enabled rejects missing certificate",
			https:     "true",
			key:       "certs/localhost.key",
			wantError: true,
		},
		{
			name:        "HTTPS enabled rejects missing key",
			https:       "true",
			certificate: "certs/localhost.crt",
			wantError:   true,
		},
		{
			name:        "certificate without key is rejected",
			certificate: "certs/localhost.crt",
			wantError:   true,
		},
		{
			name:      "key without certificate is rejected",
			key:       "certs/localhost.key",
			wantError: true,
		},
		{
			name:        "blank certificate is rejected",
			https:       "true",
			certificate: "   ",
			key:         "certs/localhost.key",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FORUM_HTTPS_ENABLED", tt.https)
			t.Setenv("FORUM_TLS_CERT_FILE", tt.certificate)
			t.Setenv("FORUM_TLS_KEY_FILE", tt.key)

			_, err := Load()

			if tt.wantError && err == nil {
				t.Fatal("Load() error = nil, want an error")
			}

			if !tt.wantError && err != nil {
				t.Fatalf("Load() returned unexpected error: %v", err)
			}
		})
	}
}
func TestLoadDerivesSecureCookiePolicy(t *testing.T) {
	tests := []struct {
		name             string
		httpsEnabled     string
		secureCookie     string
		wantSecureCookie bool
	}{
		{
			name:             "HTTP with insecure cookie configuration",
			httpsEnabled:     "false",
			secureCookie:     "false",
			wantSecureCookie: false,
		},
		{
			name:             "HTTP with explicitly secure cookie",
			httpsEnabled:     "false",
			secureCookie:     "true",
			wantSecureCookie: true,
		},
		{
			name:             "HTTPS forces secure cookie",
			httpsEnabled:     "true",
			secureCookie:     "false",
			wantSecureCookie: true,
		},
		{
			name:             "HTTPS preserves secure cookie",
			httpsEnabled:     "true",
			secureCookie:     "true",
			wantSecureCookie: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(
				"FORUM_HTTPS_ENABLED",
				tt.httpsEnabled,
			)
			t.Setenv(
				"FORUM_SECURE_COOKIE",
				tt.secureCookie,
			)

			if tt.httpsEnabled == "true" {
				t.Setenv(
					"FORUM_TLS_CERT_FILE",
					"certs/localhost.crt",
				)
				t.Setenv(
					"FORUM_TLS_KEY_FILE",
					"certs/localhost.key",
				)
			} else {
				t.Setenv("FORUM_TLS_CERT_FILE", "")
				t.Setenv("FORUM_TLS_KEY_FILE", "")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf(
					"Load() returned unexpected error: %v",
					err,
				)
			}

			if cfg.SecureCookie != tt.wantSecureCookie {
				t.Errorf(
					"SecureCookie = %t, want %t",
					cfg.SecureCookie,
					tt.wantSecureCookie,
				)
			}
		})
	}
}
