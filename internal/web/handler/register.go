// Package handler translates HTTP requests into validated service calls and
// renders user-safe responses without containing SQL.
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/validation"
	"forum/internal/web/view"
)

// RegistrationService is the account-creation behavior required by this handler.
type RegistrationService interface {
	Register(input validation.RegistrationInput) (int64, error)
}

func parseAge(value string) int {
	age, _ := strconv.Atoi(value)
	return age
}

// RegisterHandler serves the registration form and account submission.
type RegisterHandler struct {
	service  RegistrationService
	renderer *view.Renderer
}

type registerPageData struct {
	Email    string
	Username string
	Error    string
}

// NewRegisterHandler constructs registration HTTP behavior.
func NewRegisterHandler(
	service RegistrationService,
	renderer *view.Renderer,
) *RegisterHandler {
	return &RegisterHandler{
		service:  service,
		renderer: renderer,
	}
}

// ServeHTTP renders GET requests and maps POST outcomes to audit-required status
// codes without exposing persistence errors.
func (h *RegisterHandler) ServeHTTP(
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

func (h *RegisterHandler) handleGet(w http.ResponseWriter) {
	if err := h.renderer.Render(
		w,
		http.StatusOK,
		"register.html",
		registerPageData{},
	); err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
	}
}

func (h *RegisterHandler) handlePost(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := r.ParseForm(); err != nil {
		h.renderForm(
			w,
			http.StatusBadRequest,
			registerPageData{
				Error: "Invalid form",
			},
		)
		return
	}

	input := validation.RegistrationInput{
		Email:     r.FormValue("email"),
		Username:  r.FormValue("username"),
		Age:       parseAge(r.FormValue("age")),
		Gender:    r.FormValue("gender"),
		FirstName: r.FormValue("first_name"),
		LastName:  r.FormValue("last_name"),
		Password:  r.FormValue("password"),
	}

	_, err := h.service.Register(input)
	if err == nil {
		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)
		return
	}

	data := registerPageData{
		Email:    input.Email,
		Username: input.Username,
	}

	switch {
	case errors.Is(err, repository.ErrEmailExists),
		errors.Is(err, repository.ErrUsernameExists):

		data.Error = err.Error()

		h.renderForm(
			w,
			http.StatusConflict,
			data,
		)

	case errors.Is(err, service.ErrInvalidRegistration):
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
}

func (h *RegisterHandler) renderForm(
	w http.ResponseWriter,
	status int,
	data registerPageData,
) {
	if err := h.renderer.Render(
		w,
		status,
		"register.html",
		data,
	); err != nil {
		fmt.Println("render register form:", err)

		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
	}
}
