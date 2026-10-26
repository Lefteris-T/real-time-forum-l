package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"forum/internal/model"
)

type fakeCategoryList struct {
	categories []model.Category
	err        error
}

func (f *fakeCategoryList) All() ([]model.Category, error) {
	return f.categories, f.err
}

func TestCategoryListHandlerReturnsJSON(t *testing.T) {
	h := NewCategoryListHandler(&fakeCategoryList{categories: []model.Category{
		{ID: 1, Name: "General"},
		{ID: 2, Name: "Go"},
	}})
	req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	var response categoryListResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []categoryResponse{{ID: 1, Name: "General"}, {ID: 2, Name: "Go"}}
	if len(response.Categories) != len(want) {
		t.Fatalf("category count = %d, want %d", len(response.Categories), len(want))
	}
	for i := range want {
		if response.Categories[i] != want[i] {
			t.Fatalf("categories[%d] = %+v, want %+v", i, response.Categories[i], want[i])
		}
	}
}

func TestCategoryListHandlerHidesRepositoryErrors(t *testing.T) {
	h := NewCategoryListHandler(&fakeCategoryList{err: errors.New("sqlite database path leaked")})
	req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	if rec.Body.String() != "{\"error\":\"Internal Server Error\"}\n" {
		t.Fatalf("body = %q, want generic JSON error", rec.Body.String())
	}
}
