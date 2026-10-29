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

type contactListFake struct {
	contacts []repository.ConversationContact
	err      error
	userID   int64
}

func (f *contactListFake) ListContacts(userID int64) ([]repository.ConversationContact, error) {
	f.userID = userID
	return f.contacts, f.err
}

type onlineSnapshotFake map[int64]struct{}

func (f onlineSnapshotFake) OnlineUserIDs() map[int64]struct{} { return f }

func TestContactListHandlerCombinesContactOrderAndPresence(t *testing.T) {
	lastMessageAt := time.Date(2026, 10, 28, 12, 30, 0, 0, time.FixedZone("local", 2*60*60))
	reader := &contactListFake{contacts: []repository.ConversationContact{
		{UserID: 8, Username: "recent", ConversationID: 12, LastMessageAt: &lastMessageAt},
		{UserID: 9, Username: "offline", ConversationID: 0},
	}}
	handler := NewContactListHandler(reader, onlineSnapshotFake{8: {}})
	req := httptest.NewRequest(http.MethodGet, "/api/contacts", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 4}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", got)
	}
	var response contactListResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode contacts: %v", err)
	}
	if reader.userID != 4 || len(response.Contacts) != 2 {
		t.Fatalf("reader user/count = %d/%d", reader.userID, len(response.Contacts))
	}
	if response.Contacts[0].ID != 8 || !response.Contacts[0].Online || response.Contacts[0].ConversationID != 12 ||
		response.Contacts[0].LastMessageAt == nil || *response.Contacts[0].LastMessageAt != "2026-10-28T10:30:00Z" {
		t.Fatalf("recent contact = %+v", response.Contacts[0])
	}
	if response.Contacts[1].ID != 9 || response.Contacts[1].Online || response.Contacts[1].ConversationID != 0 || response.Contacts[1].LastMessageAt != nil {
		t.Fatalf("offline contact = %+v", response.Contacts[1])
	}
	if strings.Contains(recorder.Body.String(), "email") {
		t.Fatalf("contact response exposed private account data: %s", recorder.Body.String())
	}
}

func TestContactListHandlerRejectsGuestsAndHidesRepositoryErrors(t *testing.T) {
	reader := &contactListFake{err: errors.New("private sqlite details")}
	handler := NewContactListHandler(reader, onlineSnapshotFake{})
	guest := httptest.NewRecorder()
	handler.ServeHTTP(guest, httptest.NewRequest(http.MethodGet, "/api/contacts", nil))
	if guest.Code != http.StatusUnauthorized || reader.userID != 0 {
		t.Fatalf("guest status/userID = %d/%d", guest.Code, reader.userID)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/contacts", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 6}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "private sqlite") {
		t.Fatalf("error status/body = %d/%q", recorder.Code, recorder.Body.String())
	}
}

func TestContactListHandlerEnforcesGET(t *testing.T) {
	handler := NewContactListHandler(&contactListFake{}, onlineSnapshotFake{})
	req := httptest.NewRequest(http.MethodPost, "/api/contacts", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 6}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("status/Allow = %d/%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}
