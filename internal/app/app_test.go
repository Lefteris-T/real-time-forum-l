package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"forum/internal/config"
)

func TestRunReturnsStartupError(t *testing.T) {
	cfg := config.Config{
		Address:         ":99999",
		DatabasePath:    t.TempDir() + "/forum.db",
		SessionDuration: time.Hour,
		CookieName:      "forum_session",
		SecureCookie:    false,
	}

	err := Run(context.Background(), cfg)

	if err == nil {
		t.Fatal("Run() error = nil, want startup error")
	}
}

func TestRunShutsDownOnContextCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error: %v", err)
	}
	defer listener.Close()

	cfg := config.Config{
		Address:         listener.Addr().String(),
		DatabasePath:    t.TempDir() + "/forum.db",
		SessionDuration: time.Hour,
		CookieName:      "forum_session",
		SecureCookie:    false,
	}

	listener.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, cfg)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error after shutdown: %v", err)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not shut down after context cancellation")
	}
}

func TestServerClosedIsNotApplicationError(t *testing.T) {
	err := normalizeServerError(http.ErrServerClosed)

	if err != nil {
		t.Fatalf("normalizeServerError() = %v, want nil", err)
	}
}

func TestOtherServerErrorsAreReturned(t *testing.T) {
	expected := errors.New("server failed")

	err := normalizeServerError(expected)

	if !errors.Is(err, expected) {
		t.Fatalf("normalizeServerError() = %v, want %v", err, expected)
	}
}

func TestBuildHandlerAppliesLoginRateLimit(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		DatabasePath:    t.TempDir() + "/forum.db",
		SessionDuration: time.Hour,
		CookieName:      "forum_session",
	}

	handler, cleanup, err := buildHandler(cfg)
	if err != nil {
		t.Fatalf("buildHandler() error: %v", err)
	}
	t.Cleanup(cleanup)

	for requestNumber := 1; requestNumber <= 6; requestNumber++ {
		form := url.Values{
			"email":    {"missing@example.com"},
			"password": {"wrong-password"},
		}

		req := httptest.NewRequest(
			http.MethodPost,
			"/login",
			strings.NewReader(form.Encode()),
		)
		req.Header.Set(
			"Content-Type",
			"application/x-www-form-urlencoded",
		)
		req.RemoteAddr = "203.0.113.10:4567"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if requestNumber <= 5 &&
			rec.Code == http.StatusTooManyRequests {
			t.Fatalf(
				"request %d was limited before burst was exhausted",
				requestNumber,
			)
		}

		if requestNumber == 6 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf(
					"sixth request status = %d, want %d",
					rec.Code,
					http.StatusTooManyRequests,
				)
			}

			if got := rec.Header().Get("Retry-After"); got != "12" {
				t.Errorf(
					"Retry-After = %q, want %q",
					got,
					"12",
				)
			}
		}
	}
}
func TestBuildHandlerAddsSecurityHeadersToAllResponses(
	t *testing.T,
) {
	t.Parallel()

	cfg := config.Config{
		DatabasePath:    t.TempDir() + "/forum.db",
		SessionDuration: time.Hour,
		CookieName:      "forum_session",
	}

	handler, cleanup, err := buildHandler(cfg)
	if err != nil {
		t.Fatalf("buildHandler() error: %v", err)
	}
	t.Cleanup(cleanup)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "HTML page",
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found error",
			path:       "/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "static stylesheet",
			path:       "/static/style.css",
			wantStatus: http.StatusOK,
		},
		{
			name:       "static root directory is hidden",
			path:       "/static/",
			wantStatus: http.StatusNotFound,
		},
	}

	wantHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				tt.path,
				nil,
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					tt.wantStatus,
				)
			}

			for name, want := range wantHeaders {
				if got := rec.Header().Get(name); got != want {
					t.Errorf(
						"%s = %q, want %q",
						name,
						got,
						want,
					)
				}
			}
		})
	}
}
