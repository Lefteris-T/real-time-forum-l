package handler

import (
	"encoding/json"
	"net/http"

	"forum/internal/web/middleware"
)

type sessionStatusResponse struct {
	Authenticated bool `json:"authenticated"`
}

// SessionStatusHandler reports whether authentication middleware resolved a
// valid session for the current request.
type SessionStatusHandler struct{}

func NewSessionStatusHandler() *SessionStatusHandler {
	return &SessionStatusHandler{}
}

func (h *SessionStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAuthJSON(w, http.StatusMethodNotAllowed, authResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}

	_, authenticated := middleware.CurrentUser(r.Context())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(sessionStatusResponse{Authenticated: authenticated})
}
