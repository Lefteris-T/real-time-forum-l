// Package config loads and validates runtime settings from the environment.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress         = ":8080"
	defaultDatabasePath    = "data/realtime-forum.db"
	defaultSessionDuration = 24 * time.Hour
	defaultCookieName      = "forum_session"
	defaultSecureCookie    = false
	defaultHTTPSEnabled    = false
)

// Config contains every runtime value required to construct the application.
type Config struct {
	Address         string
	DatabasePath    string
	SessionDuration time.Duration
	CookieName      string
	SecureCookie    bool
	HTTPSEnabled    bool
	TLSCertFile     string
	TLSKeyFile      string
}

// Load applies defaults, reads FORUM_* overrides, and validates the result.
func Load() (Config, error) {
	cfg := Config{
		Address:         envOrDefault("FORUM_ADDRESS", defaultAddress),
		DatabasePath:    envOrDefault("FORUM_DATABASE_PATH", defaultDatabasePath),
		SessionDuration: defaultSessionDuration,
		CookieName:      envOrDefault("FORUM_COOKIE_NAME", defaultCookieName),
		SecureCookie:    defaultSecureCookie,
		HTTPSEnabled:    defaultHTTPSEnabled,
		TLSCertFile:     os.Getenv("FORUM_TLS_CERT_FILE"),
		TLSKeyFile:      os.Getenv("FORUM_TLS_KEY_FILE"),
	}
	durationValue := os.Getenv("FORUM_SESSION_DURATION")
	if durationValue != "" {
		duration, err := time.ParseDuration(durationValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid session duration: %w", err)
		}

		cfg.SessionDuration = duration
	}

	secureCookieValue := os.Getenv("FORUM_SECURE_COOKIE")
	if secureCookieValue != "" {
		secureCookie, err := strconv.ParseBool(secureCookieValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid secure cookie value: %w", err)
		}

		cfg.SecureCookie = secureCookie
	}

	httpsEnabledValue := os.Getenv("FORUM_HTTPS_ENABLED")
	if httpsEnabledValue != "" {
		httpsEnabled, err := strconv.ParseBool(httpsEnabledValue)
		if err != nil {
			return Config{}, fmt.Errorf(
				"invalid HTTPS enabled value: %w",
				err,
			)
		}
		cfg.HTTPSEnabled = httpsEnabled

	}
	cfg.SecureCookie = cfg.HTTPSEnabled || cfg.SecureCookie

	if err := validate(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

func validate(cfg *Config) error {
	if err := validateAddress(cfg.Address); err != nil {
		return err
	}

	if strings.TrimSpace(cfg.DatabasePath) == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	if cfg.SessionDuration <= 0 {
		return fmt.Errorf("session duration must be greater than zero")
	}

	if err := validateCookieName(cfg.CookieName); err != nil {
		return err
	}
	if err := validateHTTPS(cfg); err != nil {
		return err
	}
	return nil
}

func validateAddress(address string) error {
	if strings.TrimSpace(address) == "" {
		return fmt.Errorf("address cannot be empty")
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", address, err)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid address port %q: %w", port, err)
	}

	if portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("address port must be between 1 and 65535")
	}

	return nil
}

func validateCookieName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("cookie name cannot be empty")
	}

	for _, character := range name {
		if character <= 32 ||
			character >= 127 ||
			character == '(' ||
			character == ')' ||
			character == '<' ||
			character == '>' ||
			character == '@' ||
			character == ',' ||
			character == ';' ||
			character == ':' ||
			character == '\\' ||
			character == '"' ||
			character == '/' ||
			character == '[' ||
			character == ']' ||
			character == '?' ||
			character == '=' ||
			character == '{' ||
			character == '}' {
			return fmt.Errorf("cookie name contains invalid character %q", character)
		}
	}

	return nil
}

func validateHTTPS(cfg *Config) error {
	hasCertificate := strings.TrimSpace(cfg.TLSCertFile) != ""
	hasPrivateKey := strings.TrimSpace(cfg.TLSKeyFile) != ""

	if hasCertificate != hasPrivateKey {
		return fmt.Errorf(
			"TLS certificate and key paths must be configured together",
		)
	}

	if cfg.HTTPSEnabled && !hasCertificate {
		return fmt.Errorf(
			"HTTPS requires TLS certificate and key paths",
		)
	}

	return nil
}
