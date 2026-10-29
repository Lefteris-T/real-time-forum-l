package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type messageHistoryFake struct {
	latestConversationID int64
	latestUserID         int64
	olderConversationID  int64
	olderUserID          int64
	cursor               repository.MessageCursor
	latest               []model.Message
	older                []model.Message
	err                  error
}

func (f *messageHistoryFake) LatestForUser(conversationID, userID int64) ([]model.Message, error) {
	f.latestConversationID, f.latestUserID = conversationID, userID
	return f.latest, f.err
}

func (f *messageHistoryFake) OlderForUser(conversationID, userID int64, cursor repository.MessageCursor) ([]model.Message, error) {
	f.olderConversationID, f.olderUserID, f.cursor = conversationID, userID, cursor
	return f.older, f.err
}

func TestMessageHistoryHandlerReturnsLatestPageForAuthenticatedUser(t *testing.T) {
	createdAt := time.Date(2026, 10, 29, 15, 4, 5, 0, time.UTC)
	reader := &messageHistoryFake{latest: []model.Message{{
		ID: 5, ConversationID: 9, SenderID: 3, SenderUsername: "alex", Body: "hello", ImageMIME: "image/png", CreatedAt: createdAt,
	}}}
	handler := NewMessageHistoryHandler(reader)
	req := httptest.NewRequest(http.MethodGet, "/api/conversations/9/messages", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 7}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK || reader.latestConversationID != 9 || reader.latestUserID != 7 {
		t.Fatalf("status and reader args = %d, %d/%d", recorder.Code, reader.latestConversationID, reader.latestUserID)
	}
	var response messageHistoryResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Messages) != 1 || response.Messages[0].Sender != "alex" || response.Messages[0].Body != "hello" || response.Messages[0].CreatedAt != "2026-10-29T15:04:05Z" || response.Messages[0].ImageURL != "/api/messages/5/image" || response.HasOlder {
		t.Fatalf("response = %+v", response)
	}
	if strings.Contains(recorder.Body.String(), "email") {
		t.Fatalf("history response exposed private account data: %s", recorder.Body.String())
	}
}

func TestMessageHistoryHandlerParsesStableOlderCursor(t *testing.T) {
	reader := &messageHistoryFake{older: make([]model.Message, repository.MessagePageSize)}
	req := httptest.NewRequest(http.MethodGet, "/api/conversations/12/messages?before_id=44&before_created_at=2026-10-29T15%3A04%3A05Z", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 6}))
	recorder := httptest.NewRecorder()
	NewMessageHistoryHandler(reader).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || reader.olderConversationID != 12 || reader.olderUserID != 6 || reader.cursor.ID != 44 || !reader.cursor.CreatedAt.Equal(time.Date(2026, 10, 29, 15, 4, 5, 0, time.UTC)) {
		t.Fatalf("status/older args = %d, %+v", recorder.Code, reader)
	}
	var response messageHistoryResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.HasOlder {
		t.Fatal("full older page should allow another page")
	}
}

func TestMessageHistoryHandlerRejectsInvalidRequestsAndHidesErrors(t *testing.T) {
	tests := []struct {
		name string
		path string
		err  error
		want int
	}{
		{name: "invalid conversation", path: "/api/conversations/nope/messages", want: http.StatusBadRequest},
		{name: "partial cursor", path: "/api/conversations/3/messages?before_id=2", want: http.StatusBadRequest},
		{name: "nonparticipant", path: "/api/conversations/3/messages", err: repository.ErrConversationNotFound, want: http.StatusNotFound},
		{name: "repository failure", path: "/api/conversations/3/messages", err: errors.New("private sqlite details"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &messageHistoryFake{err: tt.err}
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 5}))
			recorder := httptest.NewRecorder()
			NewMessageHistoryHandler(reader).ServeHTTP(recorder, req)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, tt.want, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "private sqlite") {
				t.Fatalf("response exposed internal error: %s", recorder.Body.String())
			}
		})
	}

	guestRecorder := httptest.NewRecorder()
	NewMessageHistoryHandler(&messageHistoryFake{}).ServeHTTP(guestRecorder, httptest.NewRequest(http.MethodGet, "/api/conversations/3/messages", nil))
	if guestRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("guest status = %d, want %d", guestRecorder.Code, http.StatusUnauthorized)
	}
}
