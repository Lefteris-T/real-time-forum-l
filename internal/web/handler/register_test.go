package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/validation"
)

type fakeRegistrationService struct {
	called bool
	input  validation.RegistrationInput
	id     int64
	err    error
}

func (f *fakeRegistrationService) Register(
	input validation.RegistrationInput,
) (int64, error) {
	f.called = true
	f.input = input
	return f.id, f.err
}

func TestRegisterGETRedirectsToSPAShell(t *testing.T) {
	h := NewRegisterHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want %q", got, "/")
	}
}

func TestRegisterPOSTSuccessReturnsJSONWithoutCreatingSession(t *testing.T) {
	service := &fakeRegistrationService{id: 42}
	h := NewRegisterHandler(service)
	form := strings.NewReader(
		"email=lefteris%40example.com&" +
			"username=lefteris&age=30&gender=test&" +
			"first_name=Test&last_name=User&" +
			"password=strong-password-123",
	)
	req := httptest.NewRequest(http.MethodPost, "/register", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	var response registrationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Message != "Registration successful. Please log in." {
		t.Fatalf("message = %q", response.Message)
	}
	if rec.Header().Get("Location") != "" || rec.Header().Get("Set-Cookie") != "" {
		t.Fatal("registration redirected or created a session")
	}
	if !service.called {
		t.Fatal("registration service was not called")
	}
	if service.input.Email != "lefteris@example.com" ||
		service.input.Username != "lefteris" ||
		service.input.Age != 30 ||
		service.input.Gender != "test" ||
		service.input.FirstName != "Test" ||
		service.input.LastName != "User" ||
		service.input.Password != "strong-password-123" {
		t.Fatalf("registration input = %+v", service.input)
	}
}

func TestRegisterPOSTInvalidInputReturnsJSONBadRequest(t *testing.T) {
	service := &fakeRegistrationService{
		err: fmt.Errorf("%w: email is invalid", service.ErrInvalidRegistration),
	}
	h := NewRegisterHandler(service)
	form := strings.NewReader(
		"email=not-an-email&username=lefteris&age=30&gender=test&" +
			"first_name=Test&last_name=User&password=strong-password-123",
	)
	req := httptest.NewRequest(http.MethodPost, "/register", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var response registrationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(response.Error, "email is invalid") {
		t.Fatalf("error = %q, want validation details", response.Error)
	}
	if strings.Contains(rec.Body.String(), "strong-password-123") {
		t.Fatal("response contains the submitted password")
	}
}

func TestRegisterPOSTDuplicateReturnsJSONConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "duplicate email", err: repository.ErrEmailExists},
		{name: "duplicate nickname", err: repository.ErrUsernameExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewRegisterHandler(&fakeRegistrationService{err: tt.err})
			form := strings.NewReader(
				"email=lefteris%40example.com&username=lefteris&age=30&" +
					"gender=test&first_name=Test&last_name=User&" +
					"password=strong-password-123",
			)
			req := httptest.NewRequest(http.MethodPost, "/register", form)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
			}
			var response registrationResponse
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error != tt.err.Error() {
				t.Fatalf("error = %q, want %q", response.Error, tt.err.Error())
			}
			if strings.Contains(rec.Body.String(), "strong-password-123") {
				t.Fatal("response contains the submitted password")
			}
		})
	}
}

func TestRegisterWrongMethodReturnsJSONMethodNotAllowed(t *testing.T) {
	h := NewRegisterHandler(&fakeRegistrationService{})
	req := httptest.NewRequest(http.MethodPut, "/register", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != "GET, POST" {
		t.Fatalf("Allow = %q, want %q", got, "GET, POST")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
}

func TestRegisterHandlerDoesNotExposeInternalError(t *testing.T) {
	h := NewRegisterHandler(&fakeRegistrationService{
		err: fmt.Errorf("sqlite: database is locked at /secret/path/forum.db"),
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		strings.NewReader("email=a@example.com&username=user&password=password123"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var response registrationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("error = %q, want generic server error", response.Error)
	}
	if strings.Contains(rec.Body.String(), "sqlite") ||
		strings.Contains(rec.Body.String(), "/secret/path/forum.db") {
		t.Fatal("internal database details leaked in response")
	}
}
