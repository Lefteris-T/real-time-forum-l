// Package handler translates HTTP requests into validated service calls and
// renders user-safe responses without containing SQL.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/validation"
)

// RegistrationService is the account-creation behavior required by this handler.
type RegistrationService interface {
	Register(input validation.RegistrationInput) (int64, error)
}

type registrationResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RegisterHandler accepts SPA registration requests and responds with JSON.
type RegisterHandler struct {
	service RegistrationService
}

// NewRegisterHandler constructs registration HTTP behavior.
func NewRegisterHandler(service RegistrationService) *RegisterHandler {
	return &RegisterHandler{service: service}
}

// ServeHTTP sends GET requests to the single-page application and maps POST
// outcomes to a small JSON response the client can display.
func (h *RegisterHandler) ServeHTTP(
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
		h.writeJSON(w, http.StatusMethodNotAllowed, registrationResponse{
			Error: http.StatusText(http.StatusMethodNotAllowed),
		})
	}
}

func (h *RegisterHandler) handlePost(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		h.writeJSON(w, http.StatusBadRequest, registrationResponse{
			Error: "invalid form",
		})
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
		h.writeJSON(w, http.StatusCreated, registrationResponse{
			Message: "Registration successful. Please log in.",
		})
		return
	}

	switch {
	case errors.Is(err, repository.ErrEmailExists),
		errors.Is(err, repository.ErrUsernameExists):
		h.writeJSON(w, http.StatusConflict, registrationResponse{
			Error: err.Error(),
		})

	case errors.Is(err, service.ErrInvalidRegistration):
		h.writeJSON(w, http.StatusBadRequest, registrationResponse{
			Error: err.Error(),
		})

	default:
		h.writeJSON(w, http.StatusInternalServerError, registrationResponse{
			Error: http.StatusText(http.StatusInternalServerError),
		})
	}
}

func (h *RegisterHandler) writeJSON(
	w http.ResponseWriter,
	status int,
	response registrationResponse,
) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

func parseAge(value string) int {
	age, _ := strconv.Atoi(value)
	return age
}
