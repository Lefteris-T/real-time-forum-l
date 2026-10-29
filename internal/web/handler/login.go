package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"forum/internal/model"
	"forum/internal/service"
	"forum/internal/validation"
)

// LoginService exposes credential verification and session lifecycle behavior.
type LoginService interface {
	Login(input validation.LoginInput) (model.User, error)
	CreateSession(id string, userID int64) error
	Logout(id string) error
}

// SessionManager isolates cookie mechanics from login orchestration.
type SessionManager interface {
	Create(w http.ResponseWriter) (string, error)
	Read(r *http.Request) (string, bool)
	Clear(w http.ResponseWriter)
}

// LoginHandler serves login and creates matching cookie/database sessions.
type LoginHandler struct {
	service     LoginService
	sessions    SessionManager
	connections UserConnectionCloser
}

type authResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// NewLoginHandler constructs login HTTP behavior.
func NewLoginHandler(
	service LoginService,
	sessions SessionManager,
	connections ...UserConnectionCloser,
) *LoginHandler {
	handler := &LoginHandler{
		service:  service,
		sessions: sessions,
	}
	if len(connections) > 0 {
		handler.connections = connections[0]
	}
	return handler
}

// ServeHTTP gives unknown identifiers and wrong passwords the same
// unauthorized response, preventing account enumeration.
func (h *LoginHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodGet:
		http.Redirect(w, r, "/", http.StatusSeeOther)

	case http.MethodPost:
		h.handlePost(w, r)

	default:
		w.Header().Set("Allow", "GET, POST")
		writeAuthJSON(w, http.StatusMethodNotAllowed, authResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
	}
}

func (h *LoginHandler) handlePost(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := r.ParseForm(); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, authResponse{Error: "invalid form"})
		return
	}

	input := validation.LoginInput{
		Identifier: r.FormValue("identifier"),
		Password:   r.FormValue("password"),
	}

	user, err := h.service.Login(input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeAuthJSON(w, http.StatusUnauthorized, authResponse{Error: "Invalid nickname/email or password"})

		case errors.Is(err, service.ErrInvalidLogin):
			writeAuthJSON(w, http.StatusBadRequest, authResponse{Error: err.Error()})

		default:
			writeAuthJSON(w, http.StatusInternalServerError, authResponse{Error: http.StatusText(http.StatusInternalServerError)})
		}

		return
	}

	sessionID, err := h.sessions.Create(w)
	if err != nil {
		writeAuthJSON(w, http.StatusInternalServerError, authResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}

	if err := h.service.CreateSession(
		sessionID,
		user.ID,
	); err != nil {
		h.sessions.Clear(w)

		writeAuthJSON(w, http.StatusInternalServerError, authResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	if h.connections != nil {
		h.connections.DisconnectUser(user.ID)
	}

	writeAuthJSON(w, http.StatusOK, authResponse{Message: "Login successful."})
}

func writeAuthJSON(
	w http.ResponseWriter,
	status int,
	response authResponse,
) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
