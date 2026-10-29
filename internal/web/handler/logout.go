package handler

import (
	"net/http"

	"forum/internal/web/middleware"
)

// LogoutHandler removes both server-side and browser session state.
type LogoutHandler struct {
	service     LoginService
	sessions    SessionManager
	connections UserConnectionCloser
}

// NewLogoutHandler constructs logout HTTP behavior.
func NewLogoutHandler(
	service LoginService,
	sessions SessionManager,
	connections ...UserConnectionCloser,
) *LogoutHandler {
	handler := &LogoutHandler{
		service:  service,
		sessions: sessions,
	}
	if len(connections) > 0 {
		handler.connections = connections[0]
	}
	return handler
}

// ServeHTTP is POST-only so navigation and crawlers cannot mutate sessions.
func (h *LogoutHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeAuthJSON(w, http.StatusMethodNotAllowed, authResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}

	sessionID, ok := h.sessions.Read(r)
	if ok {
		if err := h.service.Logout(sessionID); err != nil {
			writeAuthJSON(w, http.StatusInternalServerError, authResponse{Error: http.StatusText(http.StatusInternalServerError)})
			return
		}
		if user, authenticated := middleware.CurrentUser(r.Context()); authenticated && h.connections != nil {
			h.connections.DisconnectUser(user.ID)
		}
	}

	h.sessions.Clear(w)
	writeAuthJSON(w, http.StatusOK, authResponse{Message: "Logged out."})
}
