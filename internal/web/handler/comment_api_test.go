package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/web/middleware"
)

func TestCommentAPIHandlerCreatesCommentForCurrentUser(t *testing.T) {
	service := &fakeCommentService{commentID: 91}
	req := httptest.NewRequest(http.MethodPost, "/api/posts/42/comments", strings.NewReader("body=Nice+post"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 12}))
	recorder := httptest.NewRecorder()
	NewCommentAPIHandler(service).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated || !service.called || service.authorID != 12 || service.postID != 42 || service.input.Body != "Nice post" {
		t.Fatalf("status/call = %d/%+v", recorder.Code, service)
	}
	var response commentAPIResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.CommentID != 91 || response.Message != "Comment added." {
		t.Fatalf("response = %+v", response)
	}
}

func TestCommentAPIHandlerRejectsGuestsAndHidesInternalErrors(t *testing.T) {
	service := &fakeCommentService{err: errors.New("private database details")}
	h := NewCommentAPIHandler(service)
	guest := httptest.NewRecorder()
	h.ServeHTTP(guest, httptest.NewRequest(http.MethodPost, "/api/posts/42/comments", strings.NewReader("body=hello")))
	if guest.Code != http.StatusUnauthorized || service.called {
		t.Fatalf("guest status/call = %d/%t", guest.Code, service.called)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/posts/42/comments", strings.NewReader("body=hello"))
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 12}))
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "private database") {
		t.Fatalf("status/body = %d/%q", recorder.Code, recorder.Body.String())
	}
}

func TestCommentAPIHandlerMapsMissingPostAndMalformedPath(t *testing.T) {
	service := &fakeCommentService{err: repository.ErrPostNotFound}
	h := NewCommentAPIHandler(service)
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/posts/42/comments", http.StatusNotFound},
		{"/api/posts/nope/comments", http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("body=hello"))
		req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 12}))
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, req)
		if recorder.Code != tc.want {
			t.Fatalf("%s status = %d, want %d", tc.path, recorder.Code, tc.want)
		}
	}
}
