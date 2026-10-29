package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type PrivateMessageImageReader interface {
	ImageForUser(messageID, userID int64) (model.MessageImage, error)
}

type MessageImageHandler struct{ images PrivateMessageImageReader }

func NewMessageImageHandler(images PrivateMessageImageReader) *MessageImageHandler {
	return &MessageImageHandler{images: images}
}

func (h *MessageImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	messageID, err := parsePrivateMessageImageID(r.URL.Path)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	image, err := h.images.ImageForUser(messageID, user.ID)
	if errors.Is(err, repository.ErrMessageImageNotFound) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if image.MIMEType != "image/jpeg" && image.MIMEType != "image/png" && image.MIMEType != "image/gif" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", image.MIMEType)
	w.Header().Set("Content-Length", strconv.Itoa(len(image.Data)))
	w.Header().Set("Content-Disposition", "inline; filename=message-image")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.Data)
}

func parsePrivateMessageImageID(path string) (int64, error) {
	const prefix, suffix = "/api/messages/", "/image"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, strconv.ErrSyntax
	}
	rawID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if rawID == "" || strings.Contains(rawID, "/") {
		return 0, strconv.ErrSyntax
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}
