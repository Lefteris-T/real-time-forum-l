package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type ContactReader interface {
	ListContacts(userID int64) ([]repository.ConversationContact, error)
}

type OnlineUserSnapshot interface {
	OnlineUserIDs() map[int64]struct{}
}

type ContactListHandler struct {
	contacts ContactReader
	online   OnlineUserSnapshot
}

func NewContactListHandler(contacts ContactReader, online OnlineUserSnapshot) *ContactListHandler {
	return &ContactListHandler{contacts: contacts, online: online}
}

type contactListResponse struct {
	Contacts []contactResponse `json:"contacts"`
	Error    string            `json:"error,omitempty"`
}

type contactResponse struct {
	ID             int64   `json:"id"`
	Username       string  `json:"username"`
	Online         bool    `json:"online"`
	ConversationID int64   `json:"conversation_id,omitempty"`
	LastMessageAt  *string `json:"last_message_at"`
}

// ServeHTTP returns contacts in repository order and attaches transient
// presence from the WebSocket hub without exposing private account fields.
func (h *ContactListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.writeJSON(w, http.StatusMethodNotAllowed, contactListResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		h.writeJSON(w, http.StatusUnauthorized, contactListResponse{Error: http.StatusText(http.StatusUnauthorized)})
		return
	}
	contacts, err := h.contacts.ListContacts(user.ID)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, contactListResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	onlineUsers := h.online.OnlineUserIDs()
	response := contactListResponse{Contacts: make([]contactResponse, 0, len(contacts))}
	for _, contact := range contacts {
		item := contactResponse{
			ID:             contact.UserID,
			Username:       contact.Username,
			ConversationID: contact.ConversationID,
		}
		_, item.Online = onlineUsers[contact.UserID]
		if contact.LastMessageAt != nil {
			lastMessageAt := contact.LastMessageAt.UTC().Format(time.RFC3339)
			item.LastMessageAt = &lastMessageAt
		}
		response.Contacts = append(response.Contacts, item)
	}
	h.writeJSON(w, http.StatusOK, response)
}

func (h *ContactListHandler) writeJSON(w http.ResponseWriter, status int, response contactListResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
