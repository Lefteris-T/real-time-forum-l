package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"forum/internal/service"
	"forum/internal/validation"
	"forum/internal/web/middleware"
)

type postAPIResponse struct {
	Message string `json:"message,omitempty"`
	PostID  int64  `json:"post_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

type postFeedResponse struct {
	Posts []postFeedItem `json:"posts"`
}

type postFeedItem struct {
	ID         int64              `json:"id"`
	Title      string             `json:"title"`
	Body       string             `json:"body"`
	CreatedAt  string             `json:"created_at"`
	Author     postFeedAuthor     `json:"author"`
	Categories []categoryResponse `json:"categories"`
	Likes      int                `json:"likes"`
	Dislikes   int                `json:"dislikes"`
}

type postFeedAuthor struct {
	Username string `json:"username"`
}

// PostAPIHandler lists feed posts and creates categorized posts for the SPA.
type PostAPIHandler struct {
	service PostCreationService
	posts   PostReader
}

func NewPostAPIHandler(service PostCreationService, posts PostReader) *PostAPIHandler {
	return &PostAPIHandler{service: service, posts: posts}
}

func (h *PostAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w)
	case http.MethodPost:
		h.handleCreate(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		h.writeJSON(w, http.StatusMethodNotAllowed, postAPIResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
	}
}

func (h *PostAPIHandler) handleList(w http.ResponseWriter) {
	posts, err := h.posts.List()
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, postAPIResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}

	response := postFeedResponse{Posts: make([]postFeedItem, 0, len(posts))}
	for _, post := range posts {
		item := postFeedItem{
			ID:        post.ID,
			Title:     post.Title,
			Body:      post.Body,
			CreatedAt: post.CreatedAt.Format(time.RFC3339),
			Author:    postFeedAuthor{Username: post.Author.Username},
			Categories: make(
				[]categoryResponse,
				0,
				len(post.Categories),
			),
			Likes:    post.Likes,
			Dislikes: post.Dislikes,
		}
		for _, category := range post.Categories {
			item.Categories = append(item.Categories, categoryResponse{
				ID:   category.ID,
				Name: category.Name,
			})
		}
		response.Posts = append(response.Posts, item)
	}

	h.writeJSON(w, http.StatusOK, response)
}

func (h *PostAPIHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		h.writeJSON(w, http.StatusUnauthorized, postAPIResponse{Error: http.StatusText(http.StatusUnauthorized)})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		h.writeJSON(w, http.StatusBadRequest, postAPIResponse{Error: "invalid form"})
		return
	}

	categoryValues := r.Form["category"]
	categoryIDs := make([]int64, 0, len(categoryValues))
	for _, value := range categoryValues {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			h.writeJSON(w, http.StatusBadRequest, postAPIResponse{Error: "category ID is invalid"})
			return
		}
		categoryIDs = append(categoryIDs, id)
	}

	postID, err := h.service.Create(user.ID, validation.PostInput{
		Title:       r.FormValue("title"),
		Body:        r.FormValue("body"),
		CategoryIDs: categoryIDs,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAuthenticationRequired):
			h.writeJSON(w, http.StatusUnauthorized, postAPIResponse{Error: http.StatusText(http.StatusUnauthorized)})
		case isPostValidationError(err), errors.Is(err, service.ErrInvalidPostCategory):
			h.writeJSON(w, http.StatusBadRequest, postAPIResponse{Error: err.Error()})
		default:
			h.writeJSON(w, http.StatusInternalServerError, postAPIResponse{Error: http.StatusText(http.StatusInternalServerError)})
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, postAPIResponse{
		Message: "Post created.",
		PostID:  postID,
	})
}

func (h *PostAPIHandler) writeJSON(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
