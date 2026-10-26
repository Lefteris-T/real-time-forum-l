package handler

import "net/http"

// LogoutHandler removes both server-side and browser session state.
type LogoutHandler struct {
	service  LoginService
	sessions SessionManager
}

// NewLogoutHandler constructs logout HTTP behavior.
func NewLogoutHandler(
	service LoginService,
	sessions SessionManager,
) *LogoutHandler {
	return &LogoutHandler{
		service:  service,
		sessions: sessions,
	}
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
	}

	h.sessions.Clear(w)
	writeAuthJSON(w, http.StatusOK, authResponse{Message: "Logged out."})
}
