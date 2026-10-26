package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forum/internal/model"
	"forum/internal/service"
	"forum/internal/validation"
)

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
	logoutErr           error
}

func (f *fakeLoginService) Login(input validation.LoginInput) (model.User, error) {
	f.loginCalled = true
	f.input = input
	return f.user, f.err
}

func (f *fakeLoginService) Logout(id string) error {
	f.logoutCalled = true
	f.logoutID = id
	return f.logoutErr
}

func (f *fakeLoginService) CreateSession(id string, userID int64) error {
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
	clearCalled  bool
}

func (f *fakeSessionManager) Create(w http.ResponseWriter) (string, error) {
	f.createCalled = true
	http.SetCookie(w, &http.Cookie{Name: "forum_session", Value: f.id, Path: "/", HttpOnly: true})
	return f.id, nil
}

func (f *fakeSessionManager) Read(*http.Request) (string, bool) {
	return f.readID, f.readOK
}

func (f *fakeSessionManager) Clear(w http.ResponseWriter) {
	f.clearCalled = true
	http.SetCookie(w, &http.Cookie{Name: "forum_session", Value: "", Path: "/", MaxAge: -1})
}

func TestLoginGETRedirectsToSPAShell(t *testing.T) {
	h := NewLoginHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("GET /login response = %d, Location %q; want 303 to /", rec.Code, rec.Header().Get("Location"))
	}
}

func TestLoginPOSTSuccessReturnsJSONAndCreatesSession(t *testing.T) {
	loginService := &fakeLoginService{user: model.User{ID: 42}}
	sessions := &fakeSessionManager{id: "session-123"}
	h := NewLoginHandler(loginService, sessions)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("identifier=user%40example.com&password=secret-password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	assertJSONContentType(t, rec)
	var response authResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Message != "Login successful." {
		t.Fatalf("message = %q", response.Message)
	}
	if loginService.input.Identifier != "user@example.com" || loginService.input.Password != "secret-password" {
		t.Fatalf("Login() input = %+v", loginService.input)
	}
	if !sessions.createCalled || !loginService.createSessionCalled || loginService.userID != 42 || loginService.sessionID != sessions.id {
		t.Fatal("successful login did not create the expected server session")
	}
	if rec.Header().Get("Set-Cookie") == "" {
		t.Fatal("login response did not set a session cookie")
	}
	if rec.Header().Get("Location") != "" {
		t.Fatal("login response redirected instead of returning JSON")
	}
}

func TestLoginPOSTInvalidInputReturnsJSONBadRequest(t *testing.T) {
	loginService := &fakeLoginService{err: fmt.Errorf("%w: identifier is invalid", service.ErrInvalidLogin)}
	h := NewLoginHandler(loginService, &fakeSessionManager{})
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("identifier=bad&password=secret-password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONContentType(t, rec)
	var response authResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(response.Error, "identifier is invalid") {
		t.Fatalf("error = %q, want validation detail", response.Error)
	}
	if strings.Contains(rec.Body.String(), "secret-password") {
		t.Fatal("response exposed the submitted password")
	}
}

func TestLoginPOSTInvalidCredentialsReturnsGenericJSONUnauthorized(t *testing.T) {
	loginService := &fakeLoginService{err: service.ErrInvalidCredentials}
	h := NewLoginHandler(loginService, &fakeSessionManager{})
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("identifier=unknown%40example.com&password=wrong-password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertJSONContentType(t, rec)
	var response authResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != "Invalid nickname/email or password" {
		t.Fatalf("error = %q, want generic credential error", response.Error)
	}
	if strings.Contains(rec.Body.String(), "wrong-password") {
		t.Fatal("response exposed the submitted password")
	}
	if loginService.createSessionCalled || loginService.logoutCalled {
		t.Fatal("invalid credentials unexpectedly changed session state")
	}
}

func TestLogoutPOSTReturnsJSONAndClearsSession(t *testing.T) {
	loginService := &fakeLoginService{}
	sessions := &fakeSessionManager{readID: "session-123", readOK: true}
	h := NewLogoutHandler(loginService, sessions)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	assertJSONContentType(t, rec)
	var response authResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Message != "Logged out." {
		t.Fatalf("message = %q", response.Message)
	}
	if !loginService.logoutCalled || loginService.logoutID != sessions.readID {
		t.Fatal("logout did not revoke the server-side session")
	}
	if !sessions.clearCalled {
		t.Fatal("logout did not clear the session cookie")
	}
	if rec.Header().Get("Location") != "" {
		t.Fatal("logout response redirected instead of returning JSON")
	}
}

func TestLogoutPOSTWithoutSessionIsIdempotent(t *testing.T) {
	loginService := &fakeLoginService{}
	sessions := &fakeSessionManager{}
	h := NewLogoutHandler(loginService, sessions)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !sessions.clearCalled {
		t.Fatalf("guest logout response = %d, cookie cleared = %t", rec.Code, sessions.clearCalled)
	}
	if loginService.logoutCalled {
		t.Fatal("logout attempted to revoke a missing session")
	}
}

func TestLogoutGETReturnsMethodNotAllowed(t *testing.T) {
	h := NewLogoutHandler(&fakeLoginService{}, &fakeSessionManager{})
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("GET /logout response = %d, Allow %q", rec.Code, rec.Header().Get("Allow"))
	}
	assertJSONContentType(t, rec)
}

func assertJSONContentType(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
}
