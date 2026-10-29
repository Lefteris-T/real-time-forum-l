package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forum/internal/repository"
)

type postDetailReader interface {
	Detail(id int64) (repository.PostDetail, error)
}

type PostDetailAPIHandler struct{ posts postDetailReader }

func NewPostDetailAPIHandler(posts postDetailReader) *PostDetailAPIHandler {
	return &PostDetailAPIHandler{posts: posts}
}

func (h *PostDetailAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writePostJSON(w, http.StatusMethodNotAllowed, postAPIResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
	id, err := parsePostDetailAPIPostID(r.URL.Path)
	if err != nil {
		writePostJSON(w, http.StatusBadRequest, postAPIResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}
	post, err := h.posts.Detail(id)
	if errors.Is(err, repository.ErrPostNotFound) {
		writePostJSON(w, http.StatusNotFound, postAPIResponse{Error: http.StatusText(http.StatusNotFound)})
		return
	}
	if err != nil {
		writePostJSON(w, http.StatusInternalServerError, postAPIResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	response := postDetailResponse{
		ID: post.ID, Title: post.Title, Body: post.Body,
		CreatedAt: post.CreatedAt.Format(time.RFC3339),
		Author:    postFeedAuthor{Username: post.Author.Username}, Likes: post.Likes, Dislikes: post.Dislikes,
		Categories: make([]categoryResponse, 0, len(post.Categories)),
		Comments:   make([]postCommentItem, 0, len(post.Comments)),
	}
	for _, category := range post.Categories {
		response.Categories = append(response.Categories, categoryResponse{ID: category.ID, Name: category.Name})
	}
	for _, comment := range post.Comments {
		response.Comments = append(response.Comments, postCommentItem{
			ID: comment.ID, Body: comment.Body, CreatedAt: comment.CreatedAt.Format(time.RFC3339),
			Author: postFeedAuthor{Username: comment.Author.Username}, Likes: comment.Likes, Dislikes: comment.Dislikes,
		})
	}
	writePostJSON(w, http.StatusOK, response)
}

func parsePostDetailAPIPostID(path string) (int64, error) {
	const prefix = "/api/posts/"
	if !strings.HasPrefix(path, prefix) {
		return 0, strconv.ErrSyntax
	}
	raw := strings.TrimPrefix(path, prefix)
	if raw == "" || strings.Contains(raw, "/") {
		return 0, strconv.ErrSyntax
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

type postDetailResponse struct {
	ID         int64              `json:"id"`
	Title      string             `json:"title"`
	Body       string             `json:"body"`
	CreatedAt  string             `json:"created_at"`
	Author     postFeedAuthor     `json:"author"`
	Categories []categoryResponse `json:"categories"`
	Likes      int                `json:"likes"`
	Dislikes   int                `json:"dislikes"`
	Comments   []postCommentItem  `json:"comments"`
}

type postCommentItem struct {
	ID        int64          `json:"id"`
	Body      string         `json:"body"`
	CreatedAt string         `json:"created_at"`
	Author    postFeedAuthor `json:"author"`
	Likes     int            `json:"likes"`
	Dislikes  int            `json:"dislikes"`
}

func writePostJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
