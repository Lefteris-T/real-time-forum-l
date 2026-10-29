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

func TestGuestAccessBoundaries(t *testing.T) {
	handler, cleanup, err := buildHandler(config.Config{
		DatabasePath:    t.TempDir() + "/forum.db",
		SessionDuration: time.Hour,
		CookieName:      "forum_session",
	})
	if err != nil {
		t.Fatalf("buildHandler() error: %v", err)
	}
	t.Cleanup(cleanup)

	tests := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "SPA shell", method: http.MethodGet, path: "/", status: http.StatusOK},
		{name: "SPA stylesheet", method: http.MethodGet, path: "/static/app.css", status: http.StatusOK},
		{name: "login route returns to SPA shell", method: http.MethodGet, path: "/login", status: http.StatusSeeOther},
		{name: "registration route returns to SPA shell", method: http.MethodGet, path: "/register", status: http.StatusSeeOther},
		{name: "login endpoint", method: http.MethodPost, path: "/login", status: http.StatusBadRequest},
		{name: "registration endpoint", method: http.MethodPost, path: "/register", status: http.StatusBadRequest},
		{name: "category list requires authentication", method: http.MethodGet, path: "/api/categories", status: http.StatusUnauthorized},
		{name: "contact list requires authentication", method: http.MethodGet, path: "/api/contacts", status: http.StatusUnauthorized},
		{name: "private message history requires authentication", method: http.MethodGet, path: "/api/conversations/1/messages", status: http.StatusUnauthorized},
		{name: "marking messages read requires authentication", method: http.MethodPost, path: "/api/conversations/1/read", status: http.StatusUnauthorized},
		{name: "sending a private message requires authentication", method: http.MethodPost, path: "/api/messages", status: http.StatusUnauthorized},
		{name: "private message images require authentication", method: http.MethodGet, path: "/api/messages/1/image", status: http.StatusUnauthorized},
		{name: "own profile requires authentication", method: http.MethodGet, path: "/api/profile", status: http.StatusUnauthorized},
		{name: "public profile requires authentication", method: http.MethodGet, path: "/api/users/1/profile", status: http.StatusUnauthorized},
		{name: "API post creation requires authentication", method: http.MethodPost, path: "/api/posts", status: http.StatusUnauthorized},
		{name: "API feed requires authentication", method: http.MethodGet, path: "/api/posts", status: http.StatusUnauthorized},
		{name: "logout endpoint clears guest cookie", method: http.MethodPost, path: "/logout", status: http.StatusOK},
		{name: "new post form", method: http.MethodGet, path: "/posts/new", status: http.StatusUnauthorized},
		{name: "create post", method: http.MethodPost, path: "/posts", status: http.StatusUnauthorized},
		{name: "post detail", method: http.MethodGet, path: "/posts/1", status: http.StatusUnauthorized},
		{name: "create comment", method: http.MethodPost, path: "/posts/1/comments", status: http.StatusUnauthorized},
		{name: "react to post", method: http.MethodPost, path: "/posts/1/react", status: http.StatusUnauthorized},
		{name: "react to comment", method: http.MethodPost, path: "/comments/1/react", status: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != tt.status {
				t.Fatalf(
					"status = %d, want %d",
					response.Code,
					tt.status,
				)
			}
		})
	}
}

func TestRegistrationSPAShellIncludesAllRequiredFields(t *testing.T) {
	handler, cleanup, err := buildHandler(config.Config{
		DatabasePath:    t.TempDir() + "/forum.db",
		SessionDuration: time.Hour,
		CookieName:      "forum_session",
	})
	if err != nil {
		t.Fatalf("buildHandler() error: %v", err)
	}
	t.Cleanup(cleanup)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, field := range []string{
		`name="username"`,
		`name="age"`,
		`name="gender"`,
		`name="first_name"`,
		`name="last_name"`,
		`name="email"`,
		`name="password"`,
		`data-registration-result`,
	} {
		if !strings.Contains(response.Body.String(), field) {
			t.Errorf("SPA shell does not include %s", field)
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
