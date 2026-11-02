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

// PostReactionService applies the post reaction state transition.
type PostReactionService interface {
	SetPostReaction(
		userID int64,
		postID int64,
		value model.Reaction,
	) error
}

// PostReactionHandler handles authenticated like/dislike submissions for posts.
type PostReactionHandler struct {
	service PostReactionService
}

// NewPostReactionHandler constructs post-reaction HTTP behavior.
func NewPostReactionHandler(
	service PostReactionService,
) *PostReactionHandler {
	return &PostReactionHandler{
		service: service,
	}
}

// ServeHTTP accepts only valid POST form values and redirects after mutation.
func (h *PostReactionHandler) ServeHTTP(
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

	postID, err := parseReactionPostID(r.URL.Path)
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

	rawValue := r.FormValue("value")

	valueInt, err := strconv.Atoi(rawValue)
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

	err = h.service.SetPostReaction(
		user.ID,
		postID,
		value,
	)

	if errors.Is(err, repository.ErrPostNotFound) {
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

	http.Redirect(
		w,
		r,
		"/posts/"+strconv.FormatInt(postID, 10),
		http.StatusSeeOther,
	)
}

func parseReactionPostID(
	path string,
) (int64, error) {
	const prefix = "/posts/"
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
