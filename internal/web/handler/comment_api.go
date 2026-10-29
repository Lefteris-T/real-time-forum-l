package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"forum/internal/repository"
	"forum/internal/validation"
	"forum/internal/web/middleware"
)

// CommentAPIHandler creates a comment from the authenticated SPA session.
type CommentAPIHandler struct{ service CommentService }

type commentAPIResponse struct {
	Message   string `json:"message,omitempty"`
	CommentID int64  `json:"comment_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

func NewCommentAPIHandler(service CommentService) *CommentAPIHandler {
	return &CommentAPIHandler{service: service}
}

func (h *CommentAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCommentJSON(w, http.StatusMethodNotAllowed, commentAPIResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		writeCommentJSON(w, http.StatusUnauthorized, commentAPIResponse{Error: http.StatusText(http.StatusUnauthorized)})
		return
	}
	id, err := parseCommentAPIPostID(r.URL.Path)
	if err != nil {
		writeCommentJSON(w, http.StatusBadRequest, commentAPIResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		writeCommentJSON(w, http.StatusBadRequest, commentAPIResponse{Error: "invalid form"})
		return
	}
	commentID, err := h.service.Create(user.ID, id, validation.CommentInput{Body: r.FormValue("body")})
	if errors.Is(err, repository.ErrPostNotFound) {
		writeCommentJSON(w, http.StatusNotFound, commentAPIResponse{Error: http.StatusText(http.StatusNotFound)})
		return
	}
	if errors.Is(err, validation.ErrCommentBodyRequired) || errors.Is(err, validation.ErrCommentBodyTooLong) {
		writeCommentJSON(w, http.StatusBadRequest, commentAPIResponse{Error: err.Error()})
		return
	}
	if err != nil {
		writeCommentJSON(w, http.StatusInternalServerError, commentAPIResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	writeCommentJSON(w, http.StatusCreated, commentAPIResponse{Message: "Comment added.", CommentID: commentID})
}

func writeCommentJSON(w http.ResponseWriter, status int, value commentAPIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func parseCommentAPIPostID(path string) (int64, error) {
	const prefix, suffix = "/api/posts/", "/comments"
	if len(path) <= len(prefix)+len(suffix) || path[:len(prefix)] != prefix || path[len(path)-len(suffix):] != suffix {
		return 0, strconv.ErrSyntax
	}
	raw := path[len(prefix) : len(path)-len(suffix)]
	if raw == "" || containsSlash(raw) {
		return 0, strconv.ErrSyntax
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func containsSlash(s string) bool {
	for _, r := range s {
		if r == '/' {
			return true
		}
	}
	return false
}
