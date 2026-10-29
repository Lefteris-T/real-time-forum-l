package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/web/middleware"
)

type PrivateMessageSender interface {
	Send(senderID, recipientID int64, body string) (model.Message, error)
}

type MessageSendHandler struct{ messages PrivateMessageSender }

type messageSendResponse struct {
	Message *messageHistoryItem `json:"message,omitempty"`
	Error   string              `json:"error,omitempty"`
}

func NewMessageSendHandler(messages PrivateMessageSender) *MessageSendHandler {
	return &MessageSendHandler{messages: messages}
}

func (h *MessageSendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		h.writeJSON(w, http.StatusMethodNotAllowed, messageSendResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		h.writeJSON(w, http.StatusUnauthorized, messageSendResponse{Error: http.StatusText(http.StatusUnauthorized)})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		h.writeJSON(w, http.StatusBadRequest, messageSendResponse{Error: "invalid form"})
		return
	}
	recipientID, err := strconv.ParseInt(r.FormValue("recipient_id"), 10, 64)
	if err != nil || recipientID <= 0 {
		h.writeJSON(w, http.StatusBadRequest, messageSendResponse{Error: service.ErrInvalidMessageRecipient.Error()})
		return
	}
	message, err := h.messages.Send(user.ID, recipientID, r.FormValue("body"))
	switch {
	case errors.Is(err, service.ErrInvalidMessageRecipient):
		h.writeJSON(w, http.StatusBadRequest, messageSendResponse{Error: err.Error()})
		return
	case errors.Is(err, service.ErrInvalidMessageBody):
		h.writeJSON(w, http.StatusBadRequest, messageSendResponse{Error: "message must contain 1 to 2000 characters"})
		return
	case errors.Is(err, repository.ErrUserNotFound):
		h.writeJSON(w, http.StatusNotFound, messageSendResponse{Error: http.StatusText(http.StatusNotFound)})
		return
	case err != nil:
		h.writeJSON(w, http.StatusInternalServerError, messageSendResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	item := &messageHistoryItem{
		ID: message.ID, ConversationID: message.ConversationID, SenderID: message.SenderID,
		Sender: message.SenderUsername, Body: message.Body, CreatedAt: message.CreatedAt.Format(time.RFC3339),
	}
	h.writeJSON(w, http.StatusCreated, messageSendResponse{Message: item})
}

func (h *MessageSendHandler) writeJSON(w http.ResponseWriter, status int, response messageSendResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
