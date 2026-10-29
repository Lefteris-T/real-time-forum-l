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
)

type detailAPIFake struct {
	post repository.PostDetail
	err  error
	id   int64
}

func (f *detailAPIFake) Detail(id int64) (repository.PostDetail, error) {
	f.id = id
	return f.post, f.err
}

func TestPostDetailAPIHandlerReturnsPostAndComments(t *testing.T) {
	posts := &detailAPIFake{post: repository.PostDetail{
		ID: 27, Title: "A detail", Body: "Post body",
		CreatedAt: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC),
		Author:    model.User{Username: "author", Email: "hidden@example.com"},
		Comments: []repository.CommentView{{
			ID: 8, Body: "A reply", CreatedAt: time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC),
			Author: model.User{Username: "replyer", Email: "private@example.com"}, Likes: 2,
		}},
	}}
	recorder := httptest.NewRecorder()
	NewPostDetailAPIHandler(posts).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/posts/27", nil))
	if recorder.Code != http.StatusOK || posts.id != 27 {
		t.Fatalf("status/id = %d/%d", recorder.Code, posts.id)
	}
	var response postDetailResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Title != "A detail" || len(response.Comments) != 1 || response.Comments[0].Body != "A reply" || response.Comments[0].Likes != 2 {
		t.Fatalf("response = %+v", response)
	}
	if strings.Contains(recorder.Body.String(), "@example.com") {
		t.Fatal("response exposed an email address")
	}
}

func TestPostDetailAPIHandlerMapsErrorsSafely(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{repository.ErrPostNotFound, http.StatusNotFound},
		{errors.New("private sqlite details"), http.StatusInternalServerError},
	} {
		recorder := httptest.NewRecorder()
		NewPostDetailAPIHandler(&detailAPIFake{err: tc.err}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/posts/4", nil))
		if recorder.Code != tc.want || strings.Contains(recorder.Body.String(), "private sqlite") {
			t.Fatalf("status/body = %d/%q", recorder.Code, recorder.Body.String())
		}
	}
}
