package service

import (
	"errors"

	"forum/internal/repository"
	"forum/internal/validation"
)

// ErrAuthenticationRequired is shared by protected content operations.
var ErrAuthenticationRequired = errors.New("authentication required")

// ErrInvalidPostCategory indicates that a submitted category is not in the
// current category catalogue.
var ErrInvalidPostCategory = errors.New("selected category is invalid")

type PostCreator interface {
	Create(
		authorID int64,
		title string,
		body string,
		categoryIDs []int64,
	) (int64, error)
}

type PostCategoryValidator interface {
	ValidateIDs(ids []int64) error
}

// PostService validates post creation before delegating persistence.
type PostService struct {
	posts      PostCreator
	categories PostCategoryValidator
}

func NewPostService(
	posts PostCreator,
	categories PostCategoryValidator,
) *PostService {
	return &PostService{
		posts:      posts,
		categories: categories,
	}
}

// Create requires an authenticated author and validated post input.
func (s *PostService) Create(
	authorID int64,
	input validation.PostInput,
) (int64, error) {
	if authorID <= 0 {
		return 0, ErrAuthenticationRequired
	}

	validated, err := validation.ValidatePost(input)
	if err != nil {
		return 0, err
	}
	if err := s.categories.ValidateIDs(validated.CategoryIDs); err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return 0, ErrInvalidPostCategory
		}
		return 0, err
	}

	return s.posts.Create(
		authorID,
		validated.Title,
		validated.Body,
		validated.CategoryIDs,
	)
}
