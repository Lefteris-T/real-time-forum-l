package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"forum/internal/model"
	"forum/internal/service"
	"forum/internal/web/middleware"
)

type messageSendFake struct {
	senderID, recipientID int64
	body                  string
	message               model.Message
	err                   error
}

func (f *messageSendFake) Send(senderID, recipientID int64, body string) (model.Message, error) {
	f.senderID, f.recipientID, f.body = senderID, recipientID, body
	return f.message, f.err
}

func TestMessageSendHandlerReturnsPersistedMessage(t *testing.T) {
	createdAt := time.Date(2026, 10, 29, 17, 5, 0, 0, time.UTC)
	sender := &messageSendFake{message: model.Message{ID: 4, ConversationID: 8, SenderID: 3, SenderUsername: "alex", Body: "hello", CreatedAt: createdAt}}
	form := url.Values{"recipient_id": {"9"}, "body": {"hello"}}
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 3}))
	recorder := httptest.NewRecorder()
	NewMessageSendHandler(sender).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated || sender.senderID != 3 || sender.recipientID != 9 || sender.body != "hello" {
		t.Fatalf("status/call = %d, %d/%d/%q", recorder.Code, sender.senderID, sender.recipientID, sender.body)
	}
	var response messageSendResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Message == nil || response.Message.ID != 4 || response.Message.ConversationID != 8 || response.Message.Sender != "alex" || response.Message.CreatedAt != createdAt.Format(time.RFC3339) {
		t.Fatalf("response = %+v", response)
	}
}

func TestMessageSendHandlerRejectsGuestsAndInvalidRecipient(t *testing.T) {
	sender := &messageSendFake{}
	guest := httptest.NewRecorder()
	NewMessageSendHandler(sender).ServeHTTP(guest, httptest.NewRequest(http.MethodPost, "/api/messages", nil))
	if guest.Code != http.StatusUnauthorized {
		t.Fatalf("guest status = %d, want %d", guest.Code, http.StatusUnauthorized)
	}

	form := url.Values{"recipient_id": {"3"}, "body": {"hello"}}
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 3}))
	recorder := httptest.NewRecorder()
	NewMessageSendHandler(&messageSendFake{err: service.ErrInvalidMessageRecipient}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid recipient status = %d", recorder.Code)
	}
}
