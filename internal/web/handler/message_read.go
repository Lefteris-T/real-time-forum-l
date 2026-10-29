package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type ConversationReaderMarker interface {
	MarkReadForUser(conversationID, userID int64) error
}

type MessageReadHandler struct{ messages ConversationReaderMarker }

type messageReadResponse struct {
	Error string `json:"error,omitempty"`
}

func NewMessageReadHandler(messages ConversationReaderMarker) *MessageReadHandler {
	return &MessageReadHandler{messages: messages}
}

func (h *MessageReadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		h.writeJSON(w, http.StatusMethodNotAllowed, messageReadResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		h.writeJSON(w, http.StatusUnauthorized, messageReadResponse{Error: http.StatusText(http.StatusUnauthorized)})
		return
	}
	conversationID, err := parseMessageReadConversationID(r.URL.Path)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, messageReadResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}
	if err := h.messages.MarkReadForUser(conversationID, user.ID); errors.Is(err, repository.ErrConversationNotFound) {
		h.writeJSON(w, http.StatusNotFound, messageReadResponse{Error: http.StatusText(http.StatusNotFound)})
		return
	} else if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, messageReadResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	h.writeJSON(w, http.StatusOK, messageReadResponse{})
}

func parseMessageReadConversationID(path string) (int64, error) {
	const prefix, suffix = "/api/conversations/", "/read"
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

func (h *MessageReadHandler) writeJSON(w http.ResponseWriter, status int, response messageReadResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
