package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/validation"
	"forum/internal/web/view"
)

type fakeRegistrationService struct {
	called bool

	input validation.RegistrationInput

	id  int64
	err error
}

func (f *fakeRegistrationService) Register(
	input validation.RegistrationInput,
) (int64, error) {
	f.called = true
	f.input = input

	return f.id, f.err
}

func TestRegisterGET(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "register.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				<form method="post" action="/register">
					<input name="email">
					<input name="username">
					<input name="password" type="password">
					<button type="submit">Register</button>
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

	h := NewRegisterHandler(
		nil,
		renderer,
	)

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	if !strings.Contains(rec.Body.String(), "<form") {
		t.Fatalf("response does not contain register form")
	}
}
func TestRegisterPOSTSuccess(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "register.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				<form method="post" action="/register">
					<input name="email">
					<input name="username">
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

	service := &fakeRegistrationService{
		id: 42,
	}

	h := NewRegisterHandler(
		service,
		renderer,
	)

	form := strings.NewReader(
		"email=lefteris%40example.com&" +
			"username=lefteris&" +
			"password=strong-password-123",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
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

	if !service.called {
		t.Fatal("registration service was not called")
	}

	if service.input.Email != "lefteris@example.com" {
		t.Fatalf(
			"email = %q, want %q",
			service.input.Email,
			"lefteris@example.com",
		)
	}

	if service.input.Username != "lefteris" {
		t.Fatalf(
			"username = %q, want %q",
			service.input.Username,
			"lefteris",
		)
	}

	if service.input.Password != "strong-password-123" {
		t.Fatalf("password was not passed correctly")
	}
}
func TestRegisterPOSTInvalidInputReturnsBadRequest(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "register.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				{{if .Error}}
					<p>{{.Error}}</p>
				{{end}}

				<form method="post" action="/register">
					<input name="email" value="{{.Email}}">
					<input name="username" value="{{.Username}}">
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

	service := &fakeRegistrationService{
		err: fmt.Errorf(
			"%w: invalid registration input",
			service.ErrInvalidRegistration,
		),
	}

	h := NewRegisterHandler(
		service,
		renderer,
	)

	form := strings.NewReader(
		"email=not-an-email&" +
			"username=lefteris&" +
			"password=strong-password-123",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
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

	body := rec.Body.String()

	if !strings.Contains(body, "not-an-email") {
		t.Fatal("email was not rendered back")
	}

	if !strings.Contains(body, "lefteris") {
		t.Fatal("username was not rendered back")
	}

	if strings.Contains(body, "strong-password-123") {
		t.Fatal("password was rendered back")
	}
}
func TestRegisterPOSTDuplicateReturnsConflict(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
	}{
		{
			name:       "duplicate email",
			serviceErr: repository.ErrEmailExists,
		},
		{
			name:       "duplicate username",
			serviceErr: repository.ErrUsernameExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			err := os.WriteFile(
				filepath.Join(dir, "register.html"),
				[]byte(`
					<!doctype html>
					<html>
					<body>
						{{if .Error}}
							<p>{{.Error}}</p>
						{{end}}

						<form method="post" action="/register">
							<input name="email" value="{{.Email}}">
							<input name="username" value="{{.Username}}">
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

			service := &fakeRegistrationService{
				err: tt.serviceErr,
			}

			h := NewRegisterHandler(
				service,
				renderer,
			)

			form := strings.NewReader(
				"email=lefteris%40example.com&" +
					"username=lefteris&" +
					"password=strong-password-123",
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/register",
				form,
			)

			req.Header.Set(
				"Content-Type",
				"application/x-www-form-urlencoded",
			)

			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					http.StatusConflict,
				)
			}

			if strings.Contains(
				rec.Body.String(),
				"strong-password-123",
			) {
				t.Fatal("password was rendered back")
			}
		})
	}
}
func TestRegisterWrongMethodReturnsMethodNotAllowed(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "register.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				<form method="post" action="/register"></form>
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

	service := &fakeRegistrationService{}

	h := NewRegisterHandler(
		service,
		renderer,
	)

	req := httptest.NewRequest(
		http.MethodPut,
		"/register",
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

	allow := rec.Header().Get("Allow")

	if allow != "GET, POST" {
		t.Fatalf(
			"Allow = %q, want %q",
			allow,
			"GET, POST",
		)
	}
}
func TestRegisterHandlerDoesNotExposeInternalError(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "register.html"),
		[]byte(`
			<!doctype html>
			<html>
			<body>
				{{if .Error}}
					<p>{{.Error}}</p>
				{{end}}
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

	service := &fakeRegistrationService{
		err: fmt.Errorf(
			"sqlite: database is locked at /secret/path/forum.db",
		),
	}

	h := NewRegisterHandler(
		service,
		renderer,
	)

	body := strings.NewReader(
		"email=a@example.com&username=user&password=password123",
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		body,
	)

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusInternalServerError,
		)
	}

	responseBody := rec.Body.String()

	if strings.Contains(responseBody, "sqlite") {
		t.Fatal("internal database error leaked to response")
	}

	if strings.Contains(
		responseBody,
		"/secret/path/forum.db",
	) {
		t.Fatal("internal path leaked to response")
	}
}
