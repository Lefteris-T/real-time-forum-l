package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type messageReadFake struct {
	conversationID int64
	userID         int64
	err            error
}

func (f *messageReadFake) MarkReadForUser(conversationID, userID int64) error {
	f.conversationID, f.userID = conversationID, userID
	return f.err
}

func TestMessageReadHandlerMarksParticipantConversationRead(t *testing.T) {
	reader := &messageReadFake{}
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/17/read", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 6}))
	recorder := httptest.NewRecorder()
	NewMessageReadHandler(reader).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || reader.conversationID != 17 || reader.userID != 6 {
		t.Fatalf("status/args = %d, %d/%d", recorder.Code, reader.conversationID, reader.userID)
	}
}

func TestMessageReadHandlerAuthorizationAndValidation(t *testing.T) {
	reader := &messageReadFake{err: repository.ErrConversationNotFound}
	handler := NewMessageReadHandler(reader)
	guest := httptest.NewRecorder()
	handler.ServeHTTP(guest, httptest.NewRequest(http.MethodPost, "/api/conversations/1/read", nil))
	if guest.Code != http.StatusUnauthorized {
		t.Fatalf("guest status = %d", guest.Code)
	}

	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{method: http.MethodGet, path: "/api/conversations/1/read", want: http.StatusMethodNotAllowed},
		{method: http.MethodPost, path: "/api/conversations/no/read", want: http.StatusBadRequest},
		{method: http.MethodPost, path: "/api/conversations/1/read", want: http.StatusNotFound},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 6}))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != tc.want {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, recorder.Code, tc.want)
		}
	}
	reader.err = errors.New("private sqlite details")
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/1/read", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 6}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError || recorder.Body.String() == "private sqlite details\n" {
		t.Fatalf("internal error response = %d/%q", recorder.Code, recorder.Body.String())
	}
}
