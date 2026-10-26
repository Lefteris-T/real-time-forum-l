package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/validation"
	"forum/internal/web/middleware"
)

func TestPostAPIHandlerCreatesPostForCurrentUser(t *testing.T) {
	postService := &fakePostCreationService{postID: 73}
	h := NewPostAPIHandler(postService, &fakePostReader{})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader(
		"title=Hello&body=Forum+post&category=2&category=4",
	))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 42}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	var response postAPIResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.PostID != 73 || response.Message != "Post created." {
		t.Fatalf("response = %+v", response)
	}
	if !postService.called || postService.authorID != 42 {
		t.Fatalf("service call = %t, authorID = %d; want current user 42", postService.called, postService.authorID)
	}
	if postService.input.Title != "Hello" || postService.input.Body != "Forum post" || len(postService.input.CategoryIDs) != 2 || postService.input.CategoryIDs[0] != 2 || postService.input.CategoryIDs[1] != 4 {
		t.Fatalf("service input = %+v", postService.input)
	}
}

func TestPostAPIHandlerListsFeedWithoutExposingAuthorEmail(t *testing.T) {
	posts := &fakePostReader{posts: []repository.PostListItem{{
		ID:        19,
		Title:     "A feed post",
		Body:      "Post body",
		CreatedAt: time.Date(2026, 10, 26, 10, 30, 0, 0, time.UTC),
		Author: model.User{
			Email:    "private@example.com",
			Username: "writer",
		},
		Categories: []model.Category{{ID: 2, Name: "Go"}},
		Likes:      3,
		Dislikes:   1,
	}}}
	h := NewPostAPIHandler(&fakePostCreationService{}, posts)
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 42}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	var response postFeedResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode feed: %v", err)
	}
	if len(response.Posts) != 1 {
		t.Fatalf("post count = %d, want 1", len(response.Posts))
	}
	post := response.Posts[0]
	if post.ID != 19 || post.Title != "A feed post" || post.Body != "Post body" {
		t.Fatalf("post = %+v", post)
	}
	if post.Author.Username != "writer" || strings.Contains(rec.Body.String(), "private@example.com") {
		t.Fatalf("author response = %+v; private email leaked", post.Author)
	}
	if post.CreatedAt != "2026-10-26T10:30:00Z" || len(post.Categories) != 1 || post.Categories[0].Name != "Go" {
		t.Fatalf("post timestamp/categories = %q / %+v", post.CreatedAt, post.Categories)
	}
	if post.Likes != 3 || post.Dislikes != 1 {
		t.Fatalf("reaction counts = %d likes, %d dislikes", post.Likes, post.Dislikes)
	}
}

func TestPostAPIHandlerReturnsEmptyFeedAsArray(t *testing.T) {
	h := NewPostAPIHandler(&fakePostCreationService{}, &fakePostReader{})
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 42}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "{\"posts\":[]}\n" {
		t.Fatalf("empty feed response = %d %q", rec.Code, rec.Body.String())
	}
}

func TestPostAPIHandlerHidesFeedRepositoryErrors(t *testing.T) {
	h := NewPostAPIHandler(&fakePostCreationService{}, &fakePostReader{err: errors.New("sqlite storage details")})
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 42}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var response postAPIResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != http.StatusText(http.StatusInternalServerError) || strings.Contains(rec.Body.String(), "sqlite storage details") {
		t.Fatalf("feed error response = %q", rec.Body.String())
	}
}

func TestPostAPIHandlerRejectsGuest(t *testing.T) {
	postService := &fakePostCreationService{}
	h := NewPostAPIHandler(postService, &fakePostReader{})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader("title=Hello&body=World&category=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if postService.called {
		t.Fatal("post service was called for a guest")
	}
}

func TestPostAPIHandlerReturnsSafeValidationAndInternalErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{
			name:       "invalid category",
			err:        service.ErrInvalidPostCategory,
			wantStatus: http.StatusBadRequest,
			wantError:  service.ErrInvalidPostCategory.Error(),
		},
		{
			name:       "invalid title",
			err:        validation.ErrPostTitleRequired,
			wantStatus: http.StatusBadRequest,
			wantError:  validation.ErrPostTitleRequired.Error(),
		},
		{
			name:       "database error",
			err:        errors.New("sqlite internal details"),
			wantStatus: http.StatusInternalServerError,
			wantError:  http.StatusText(http.StatusInternalServerError),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			postService := &fakePostCreationService{err: tt.err}
			h := NewPostAPIHandler(postService, &fakePostReader{})
			req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader("title=Hello&body=World&category=1"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 42}))
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var response postAPIResponse
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error != tt.wantError {
				t.Fatalf("error = %q, want %q", response.Error, tt.wantError)
			}
			if strings.Contains(rec.Body.String(), "sqlite internal details") {
				t.Fatal("response leaked internal database error")
			}
		})
	}
}
