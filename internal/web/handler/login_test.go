package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forum/internal/model"
	"forum/internal/service"
	"forum/internal/validation"
	"forum/internal/web/view"
)

func TestLoginGET(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "login.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				<form method="post" action="/login">
					<input name="email">
					<input name="password" type="password">
					<button type="submit">Login</button>
				</form>
			</body>
			</html>
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	renderer, err := view.NewRenderer(dir)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	h := NewLoginHandler(
		nil,
		nil,
		renderer,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/login",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusOK,
		)
	}

	if !strings.Contains(rec.Body.String(), "<form") {
		t.Fatal("response does not contain login form")
	}
}

type fakeLoginService struct {
	user model.User
	err  error

	loginCalled bool
	input       validation.LoginInput

	createSessionCalled bool
	sessionID           string
	userID              int64
	logoutCalled        bool
	logoutID            string
}

func (f *fakeLoginService) Login(
	input validation.LoginInput,
) (model.User, error) {
	f.loginCalled = true
	f.input = input

	return f.user, f.err
}
func (f *fakeLoginService) Logout(id string) error {
	f.logoutCalled = true
	f.logoutID = id

	return nil
}

func (f *fakeLoginService) CreateSession(
	id string,
	userID int64,
) error {
	f.createSessionCalled = true
	f.sessionID = id
	f.userID = userID

	return nil
}

type fakeSessionManager struct {
	id string

	createCalled bool
	readID       string
	readOK       bool

	clearCalled bool
}

func (f *fakeSessionManager) Create(
	w http.ResponseWriter,
) (string, error) {
	f.createCalled = true

	http.SetCookie(w, &http.Cookie{
		Name:     "forum_session",
		Value:    f.id,
		Path:     "/",
		HttpOnly: true,
	})

	return f.id, nil
}
func (f *fakeSessionManager) Read(
	r *http.Request,
) (string, bool) {
	return f.readID, f.readOK
}

func (f *fakeSessionManager) Clear(
	w http.ResponseWriter,
) {
	f.clearCalled = true

	http.SetCookie(w, &http.Cookie{
		Name:   "forum_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}
func TestLoginPOSTSuccess(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "login.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				<form method="post" action="/login">
					<input name="email">
					<input name="password" type="password">
				</form>
			</body>
			</html>
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	renderer, err := view.NewRenderer(dir)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	service := &fakeLoginService{
		user: model.User{
			ID:    42,
			Email: "lefteris@example.com",
		},
	}

	sessions := &fakeSessionManager{
		id: "550e8400-e29b-41d4-a716-446655440000",
	}

	h := NewLoginHandler(
		service,
		sessions,
		renderer,
	)

	form := strings.NewReader(
		"email=lefteris%40example.com&" +
			"password=strong-password-123",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		form,
	)

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusSeeOther,
		)
	}

	if !service.loginCalled {
		t.Fatal("Login() was not called")
	}

	if !sessions.createCalled {
		t.Fatal("session manager Create() was not called")
	}

	if !service.createSessionCalled {
		t.Fatal("CreateSession() was not called")
	}

	if service.userID != 42 {
		t.Fatalf(
			"session userID = %d, want 42",
			service.userID,
		)
	}

	if service.sessionID != sessions.id {
		t.Fatalf(
			"session id = %q, want %q",
			service.sessionID,
			sessions.id,
		)
	}

	if rec.Header().Get("Set-Cookie") == "" {
		t.Fatal("login response did not set cookie")
	}
}
func TestLoginPOSTInvalidInputReturnsBadRequest(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "login.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				{{if .Error}}
					<p>{{.Error}}</p>
				{{end}}

				<form method="post" action="/login">
					<input name="email" value="{{.Email}}">
					<input name="password" type="password">
				</form>
			</body>
			</html>
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	renderer, err := view.NewRenderer(dir)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	loginService := &fakeLoginService{
		err: fmt.Errorf(
			"%w: invalid email",
			service.ErrInvalidLogin,
		),
	}

	sessions := &fakeSessionManager{
		id: "550e8400-e29b-41d4-a716-446655440000",
	}

	h := NewLoginHandler(
		loginService,
		sessions,
		renderer,
	)

	form := strings.NewReader(
		"email=not-an-email&" +
			"password=strong-password-123",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		form,
	)

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusBadRequest,
		)
	}

	if !strings.Contains(rec.Body.String(), "not-an-email") {
		t.Fatal("email was not rendered back")
	}

	if strings.Contains(rec.Body.String(), "strong-password-123") {
		t.Fatal("password was rendered back")
	}
}
func TestLoginPOSTWrongCredentialsReturnsUnauthorized(t *testing.T) {
	service := &fakeLoginService{
		err: service.ErrInvalidCredentials,
	}

	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "login.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				{{if .Error}}
					<p>{{.Error}}</p>
				{{end}}

				<form method="post" action="/login">
					<input name="email" value="{{.Email}}">
					<input name="password" type="password">
				</form>
			</body>
			</html>
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	renderer, err := view.NewRenderer(dir)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	sessions := &fakeSessionManager{}

	h := NewLoginHandler(
		service,
		sessions,
		renderer,
	)

	form := strings.NewReader(
		"email=lefteris%40example.com&" +
			"password=wrong-password",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		form,
	)

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "Wrong email or password") {
		t.Fatalf(
			"body does not contain safe credential message: %s",
			body,
		)
	}

	if strings.Contains(body, "wrong-password") {
		t.Fatal("password was rendered back")
	}
}
func TestLogoutPOSTClearsSession(t *testing.T) {
	service := &fakeLoginService{}

	sessions := &fakeSessionManager{
		readID: "550e8400-e29b-41d4-a716-446655440000",
		readOK: true,
	}

	h := NewLogoutHandler(
		service,
		sessions,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/logout",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusSeeOther,
		)
	}

	if !service.logoutCalled {
		t.Fatal("Logout() was not called")
	}

	if service.logoutID != sessions.readID {
		t.Fatalf(
			"logout session id = %q, want %q",
			service.logoutID,
			sessions.readID,
		)
	}

	if !sessions.clearCalled {
		t.Fatal("session cookie was not cleared")
	}
}
func TestLogoutGETReturnsMethodNotAllowed(t *testing.T) {
	service := &fakeLoginService{}
	sessions := &fakeSessionManager{}

	h := NewLogoutHandler(
		service,
		sessions,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/logout",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if rec.Header().Get("Allow") != "POST" {
		t.Fatalf(
			"Allow = %q, want POST",
			rec.Header().Get("Allow"),
		)
	}
}
