package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type profileManagerFake struct {
	profile model.Profile
	userID  int64
	update  model.ProfileUpdate
	err     error
}

func (f *profileManagerFake) Get(userID int64) (model.Profile, error) {
	f.userID = userID
	return f.profile, f.err
}

func (f *profileManagerFake) Update(userID int64, update model.ProfileUpdate) (model.Profile, error) {
	f.userID, f.update = userID, update
	return f.profile, f.err
}

func TestProfileHandlerOnlyReturnsPublicFields(t *testing.T) {
	reader := &profileManagerFake{profile: model.Profile{
		ID: 7, Username: "public-user", FirstName: "First", LastName: "Last", Age: 28,
		Gender: "private choice",
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/users/7/profile", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 3}))
	recorder := httptest.NewRecorder()
	NewProfileHandler(reader).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || reader.userID != 7 {
		t.Fatalf("status/profile ID = %d/%d", recorder.Code, reader.userID)
	}
	if strings.Contains(recorder.Body.String(), "email") || strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "private@example.com") {
		t.Fatalf("profile response exposed private account fields: %s", recorder.Body.String())
	}
	var response profileAPIResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if response.Profile == nil || response.Profile.Username != "public-user" {
		t.Fatalf("response profile = %+v", response.Profile)
	}
}

func TestProfileHandlerUpdatesOnlyAuthenticatedOwnersProfile(t *testing.T) {
	manager := &profileManagerFake{profile: model.Profile{ID: 11, Username: "owner"}}
	form := url.Values{"first_name": {"New"}, "last_name": {"Name"}, "age": {"34"}, "gender": {"other"}}
	req := httptest.NewRequest(http.MethodPut, "/api/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 11}))
	recorder := httptest.NewRecorder()
	NewProfileHandler(manager).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || manager.userID != 11 || manager.update.FirstName != "New" || manager.update.Age != 34 {
		t.Fatalf("status/owner/update = %d/%d/%+v", recorder.Code, manager.userID, manager.update)
	}

	publicEdit := httptest.NewRequest(http.MethodPut, "/api/users/12/profile", strings.NewReader(form.Encode()))
	publicEdit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	publicEdit = publicEdit.WithContext(middleware.ContextWithUser(publicEdit.Context(), model.User{ID: 11}))
	publicRecorder := httptest.NewRecorder()
	NewProfileHandler(manager).ServeHTTP(publicRecorder, publicEdit)
	if publicRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("public profile edit status = %d", publicRecorder.Code)
	}
}

func TestProfileHandlerAuthorizationValidationAndNotFound(t *testing.T) {
	handler := NewProfileHandler(&profileManagerFake{err: repository.ErrUserNotFound})
	guest := httptest.NewRecorder()
	handler.ServeHTTP(guest, httptest.NewRequest(http.MethodGet, "/api/profile", nil))
	if guest.Code != http.StatusUnauthorized {
		t.Fatalf("guest status = %d", guest.Code)
	}

	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/api/users/no/profile", want: http.StatusBadRequest},
		{path: "/api/users/90/profile", want: http.StatusNotFound},
	} {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 5}))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != test.want {
			t.Errorf("GET %s status = %d, want %d", test.path, recorder.Code, test.want)
		}
	}
}
