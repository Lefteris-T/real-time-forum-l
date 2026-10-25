package handler

import (
	"errors"
	"net/http"

	"forum/internal/model"
	"forum/internal/service"
	"forum/internal/validation"
	"forum/internal/web/view"
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
	service  LoginService
	sessions SessionManager
	renderer *view.Renderer
}

type loginPageData struct {
	Email string
	Error string
}

// NewLoginHandler constructs login HTTP behavior.
func NewLoginHandler(
	service LoginService,
	sessions SessionManager,
	renderer *view.Renderer,
) *LoginHandler {
	return &LoginHandler{
		service:  service,
		sessions: sessions,
		renderer: renderer,
	}
}

// ServeHTTP gives unknown emails and wrong passwords the same unauthorized
// response, preventing account enumeration.
func (h *LoginHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w)

	case http.MethodPost:
		h.handlePost(w, r)

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(
			w,
			http.StatusText(http.StatusMethodNotAllowed),
			http.StatusMethodNotAllowed,
		)
	}
}

func (h *LoginHandler) handleGet(w http.ResponseWriter) {
	if err := h.renderer.Render(
		w,
		http.StatusOK,
		"login.html",
		loginPageData{},
	); err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
	}
}

func (h *LoginHandler) handlePost(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := r.ParseForm(); err != nil {
		h.renderForm(
			w,
			http.StatusBadRequest,
			loginPageData{
				Error: "Invalid form",
			},
		)
		return
	}

	input := validation.LoginInput{
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
	}

	user, err := h.service.Login(input)
	if err != nil {
		data := loginPageData{
			Email: input.Email,
		}

		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			data.Error = "Wrong email or password"

			h.renderForm(
				w,
				http.StatusUnauthorized,
				data,
			)

		case errors.Is(err, service.ErrInvalidLogin):
			data.Error = err.Error()

			h.renderForm(
				w,
				http.StatusBadRequest,
				data,
			)

		default:
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
		}

		return
	}

	sessionID, err := h.sessions.Create(w)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
		return
	}

	if err := h.service.CreateSession(
		sessionID,
		user.ID,
	); err != nil {
		h.sessions.Clear(w)

		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/",
		http.StatusSeeOther,
	)
}

func (h *LoginHandler) renderForm(
	w http.ResponseWriter,
	status int,
	data loginPageData,
) {
	if err := h.renderer.Render(
		w,
		status,
		"login.html",
		data,
	); err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
	}
}
