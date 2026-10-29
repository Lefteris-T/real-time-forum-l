package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type MessageHistoryReader interface {
	LatestForUser(conversationID, userID int64) ([]model.Message, error)
	OlderForUser(conversationID, userID int64, before repository.MessageCursor) ([]model.Message, error)
}

type MessageHistoryHandler struct {
	messages MessageHistoryReader
}

func NewMessageHistoryHandler(messages MessageHistoryReader) *MessageHistoryHandler {
	return &MessageHistoryHandler{messages: messages}
}

type messageHistoryResponse struct {
	Messages []messageHistoryItem `json:"messages"`
	HasOlder bool                 `json:"has_older"`
	Error    string               `json:"error,omitempty"`
}

type messageHistoryItem struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	SenderID       int64  `json:"sender_id"`
	Sender         string `json:"sender"`
	Body           string `json:"body"`
	CreatedAt      string `json:"created_at"`
	ImageURL       string `json:"image_url,omitempty"`
}

func (h *MessageHistoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.writeJSON(w, http.StatusMethodNotAllowed, messageHistoryResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		h.writeJSON(w, http.StatusUnauthorized, messageHistoryResponse{Error: http.StatusText(http.StatusUnauthorized)})
		return
	}
	conversationID, err := parseMessageHistoryConversationID(r.URL.Path)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, messageHistoryResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}

	beforeID := r.URL.Query().Get("before_id")
	beforeCreatedAt := r.URL.Query().Get("before_created_at")
	var messages []model.Message
	if beforeID == "" && beforeCreatedAt == "" {
		messages, err = h.messages.LatestForUser(conversationID, user.ID)
	} else {
		cursor, cursorErr := parseMessageHistoryCursor(beforeID, beforeCreatedAt)
		if cursorErr != nil {
			h.writeJSON(w, http.StatusBadRequest, messageHistoryResponse{Error: http.StatusText(http.StatusBadRequest)})
			return
		}
		messages, err = h.messages.OlderForUser(conversationID, user.ID, cursor)
	}
	if errors.Is(err, repository.ErrConversationNotFound) {
		h.writeJSON(w, http.StatusNotFound, messageHistoryResponse{Error: http.StatusText(http.StatusNotFound)})
		return
	}
	if errors.Is(err, repository.ErrInvalidMessageCursor) {
		h.writeJSON(w, http.StatusBadRequest, messageHistoryResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, messageHistoryResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	response := messageHistoryResponse{
		Messages: make([]messageHistoryItem, 0, len(messages)),
		HasOlder: len(messages) == repository.MessagePageSize,
	}
	for _, message := range messages {
		item := messageHistoryItem{
			ID:             message.ID,
			ConversationID: message.ConversationID,
			SenderID:       message.SenderID,
			Sender:         message.SenderUsername,
			Body:           message.Body,
			CreatedAt:      message.CreatedAt.Format(time.RFC3339),
		}
		if message.ImageMIME != "" {
			item.ImageURL = fmt.Sprintf("/api/messages/%d/image", message.ID)
		}
		response.Messages = append(response.Messages, item)
	}
	h.writeJSON(w, http.StatusOK, response)
}

func parseMessageHistoryConversationID(path string) (int64, error) {
	const prefix, suffix = "/api/conversations/", "/messages"
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

func parseMessageHistoryCursor(beforeID, beforeCreatedAt string) (repository.MessageCursor, error) {
	if beforeID == "" || beforeCreatedAt == "" {
		return repository.MessageCursor{}, repository.ErrInvalidMessageCursor
	}
	id, err := strconv.ParseInt(beforeID, 10, 64)
	if err != nil || id <= 0 {
		return repository.MessageCursor{}, repository.ErrInvalidMessageCursor
	}
	createdAt, err := time.Parse(time.RFC3339, beforeCreatedAt)
	if err != nil {
		return repository.MessageCursor{}, repository.ErrInvalidMessageCursor
	}
	return repository.MessageCursor{ID: id, CreatedAt: createdAt}, nil
}

func (h *MessageHistoryHandler) writeJSON(w http.ResponseWriter, status int, response messageHistoryResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
