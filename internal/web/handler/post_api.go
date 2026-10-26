package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"forum/internal/service"
	"forum/internal/validation"
	"forum/internal/web/middleware"
)

type postAPIResponse struct {
	Message string `json:"message,omitempty"`
	PostID  int64  `json:"post_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// PostAPIHandler creates a categorized post and returns a JSON response for
// the SPA.
type PostAPIHandler struct {
	service PostCreationService
}

func NewPostAPIHandler(service PostCreationService) *PostAPIHandler {
	return &PostAPIHandler{service: service}
}

func (h *PostAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		h.writeJSON(w, http.StatusMethodNotAllowed, postAPIResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}

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

func (h *PostAPIHandler) writeJSON(w http.ResponseWriter, status int, response postAPIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
