package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/web/middleware"
)

// CommentReactionService applies the comment reaction state transition.
type CommentReactionService interface {
	SetCommentReaction(
		userID int64,
		commentID int64,
		value model.Reaction,
	) error
}

// CommentPostFinder resolves the parent post used for the success redirect.
type CommentPostFinder interface {
	PostIDForComment(
		commentID int64,
	) (int64, error)
}

// CommentReactionHandler handles authenticated reactions to comments.
type CommentReactionHandler struct {
	service  CommentReactionService
	comments CommentPostFinder
}

// NewCommentReactionHandler constructs comment-reaction HTTP behavior.
func NewCommentReactionHandler(
	service CommentReactionService,
	comments CommentPostFinder,
) *CommentReactionHandler {
	return &CommentReactionHandler{
		service:  service,
		comments: comments,
	}
}

// ServeHTTP mutates the reaction and returns the browser to its parent post.
func (h *CommentReactionHandler) ServeHTTP(
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

	commentID, err := parseReactionCommentID(r.URL.Path)
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

	valueInt, err := strconv.Atoi(
		r.FormValue("value"),
	)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	value := model.Reaction(valueInt)

	if value != model.ReactionLike &&
		value != model.ReactionDislike {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	err = h.service.SetCommentReaction(
		user.ID,
		commentID,
		value,
	)

	if errors.Is(err, repository.ErrCommentNotFound) {
		http.Error(
			w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound,
		)
		return
	}

	if errors.Is(err, service.ErrInvalidReaction) {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		return
	}

	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
		return
	}

	postID, err := h.comments.PostIDForComment(commentID)

	if errors.Is(err, repository.ErrCommentNotFound) {
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
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
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

func parseReactionCommentID(
	path string,
) (int64, error) {
	const prefix = "/comments/"
	const suffix = "/react"

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
