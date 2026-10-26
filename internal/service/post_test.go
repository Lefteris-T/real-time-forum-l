package service

import (
	"errors"
	"testing"

	"forum/internal/repository"
	"forum/internal/validation"
)

type fakePostCreator struct {
	called      bool
	authorID    int64
	title       string
	body        string
	categoryIDs []int64

	postID int64
	err    error
}

type fakePostCategoryValidator struct {
	called bool
	ids    []int64
	err    error
}

func (f *fakePostCategoryValidator) ValidateIDs(ids []int64) error {
	f.called = true
	f.ids = append([]int64(nil), ids...)
	return f.err
}

func (f *fakePostCreator) Create(
	authorID int64,
	title string,
	body string,
	categoryIDs []int64,
) (int64, error) {
	f.called = true
	f.authorID = authorID
	f.title = title
	f.body = body
	f.categoryIDs = categoryIDs

	return f.postID, f.err
}

func TestPostServiceRejectsGuest(t *testing.T) {
	repo := &fakePostCreator{}

	service := NewPostService(repo, &fakePostCategoryValidator{})

	input := validation.PostInput{
		Title:       "valid title",
		Body:        "valid body",
		CategoryIDs: []int64{1},
	}

	_, err := service.Create(0, input)

	if !errors.Is(err, ErrAuthenticationRequired) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrAuthenticationRequired,
		)
	}

	if repo.called {
		t.Fatal("repository Create() was called for guest")
	}
}

func TestPostServiceInvalidInputStopsBeforeRepository(t *testing.T) {
	repo := &fakePostCreator{}

	service := NewPostService(repo, &fakePostCategoryValidator{})

	input := validation.PostInput{
		Title:       "   ",
		Body:        "valid body",
		CategoryIDs: []int64{1},
	}

	_, err := service.Create(42, input)

	if err == nil {
		t.Fatal("Create() error = nil, want validation error")
	}

	if repo.called {
		t.Fatal("repository Create() was called for invalid input")
	}
}

func TestPostServiceCreatesTextOnlyPost(t *testing.T) {
	repo := &fakePostCreator{postID: 100}
	service := NewPostService(repo, &fakePostCategoryValidator{})

	input := validation.PostInput{
		Title:       "Text-only post",
		Body:        "This post has no image.",
		CategoryIDs: []int64{1},
	}

	postID, err := service.Create(42, input)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if postID != 100 {
		t.Fatalf("postID = %d, want 100", postID)
	}
	if !repo.called {
		t.Fatal("repository Create() was not called")
	}
}

func TestPostServiceUsesAuthenticatedUserAsAuthor(t *testing.T) {
	repo := &fakePostCreator{
		postID: 100,
	}

	categoryValidator := &fakePostCategoryValidator{}
	service := NewPostService(repo, categoryValidator)

	input := validation.PostInput{
		Title:       "  My post  ",
		Body:        "  My body  ",
		CategoryIDs: []int64{1, 2},
	}

	postID, err := service.Create(42, input)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if postID != 100 {
		t.Fatalf(
			"postID = %d, want 100",
			postID,
		)
	}

	if !repo.called {
		t.Fatal("repository Create() was not called")
	}
	if !categoryValidator.called || len(categoryValidator.ids) != 2 || categoryValidator.ids[0] != 1 || categoryValidator.ids[1] != 2 {
		t.Fatalf("category validation inputs = %v, want [1 2]", categoryValidator.ids)
	}

	if repo.authorID != 42 {
		t.Fatalf(
			"authorID = %d, want 42",
			repo.authorID,
		)
	}

	if repo.title != "My post" {
		t.Fatalf(
			"title = %q, want %q",
			repo.title,
			"My post",
		)
	}

	if repo.body != "My body" {
		t.Fatalf(
			"body = %q, want %q",
			repo.body,
			"My body",
		)
	}

}

func TestPostServiceRejectsUnknownCategoryBeforeRepository(t *testing.T) {
	repo := &fakePostCreator{}
	categoryValidator := &fakePostCategoryValidator{err: repository.ErrCategoryNotFound}
	service := NewPostService(repo, categoryValidator)

	_, err := service.Create(42, validation.PostInput{
		Title:       "A valid title",
		Body:        "A valid post body",
		CategoryIDs: []int64{999},
	})
	if !errors.Is(err, ErrInvalidPostCategory) {
		t.Fatalf("Create() error = %v, want %v", err, ErrInvalidPostCategory)
	}
	if repo.called {
		t.Fatal("post repository was called with an unknown category")
	}
}
