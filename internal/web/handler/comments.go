package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"forum/internal/repository"
	"forum/internal/validation"
	"forum/internal/web/middleware"
)

// CommentService is the comment write behavior required by this handler.
type CommentService interface {
	Create(
		authorID int64,
		postID int64,
		input validation.CommentInput,
	) (int64, error)
}

// CommentSubmissionHandler processes authenticated comment form submissions.
type CommentSubmissionHandler struct {
	service CommentService
}

// NewCommentSubmissionHandler constructs comment-submission HTTP behavior.
func NewCommentSubmissionHandler(
	service CommentService,
) *CommentSubmissionHandler {
	return &CommentSubmissionHandler{
		service: service,
	}
}

// ServeHTTP validates route/form identifiers before calling the service and
// redirects to the parent post on success.
func (h *CommentSubmissionHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")

		http.Error(
			w,
			http.StatusText(http.StatusMethodNotAllowed),
			http.StatusMethodNotAllowed,
		)
		return
	}

	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Error(
			w,
			http.StatusText(http.StatusUnauthorized),
			http.StatusUnauthorized,
		)
		return
	}

	postID, err := parseCommentPostID(r.URL.Path)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	input := validation.CommentInput{
		Body: r.FormValue("body"),
	}

	_, err = h.service.Create(
		user.ID,
		postID,
		input,
	)

	if errors.Is(err, repository.ErrPostNotFound) {
		http.Error(
			w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound,
		)
		return
	}

	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/posts/"+strconv.FormatInt(postID, 10),
		http.StatusSeeOther,
	)
}

func parseCommentPostID(path string) (int64, error) {
	const prefix = "/posts/"
	const suffix = "/comments"

	if !strings.HasPrefix(path, prefix) ||
		!strings.HasSuffix(path, suffix) {
		return 0, strconv.ErrSyntax
	}

	rawID := strings.TrimSuffix(
		strings.TrimPrefix(path, prefix),
		suffix,
	)

	if rawID == "" || strings.Contains(rawID, "/") {
		return 0, strconv.ErrSyntax
	}

	id, err := strconv.ParseInt(
		rawID,
		10,
		64,
	)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}

	return id, nil
}
