package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/validation"
	"forum/internal/web/middleware"
)

type ProfileManager interface {
	Get(userID int64) (model.Profile, error)
	Update(userID int64, update model.ProfileUpdate) (model.Profile, error)
}

type ProfileHandler struct{ profiles ProfileManager }

type publicProfileResponse struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Age       int    `json:"age"`
	Gender    string `json:"gender"`
}

type profileAPIResponse struct {
	Profile *publicProfileResponse `json:"profile,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

func NewProfileHandler(profiles ProfileManager) *ProfileHandler {
	return &ProfileHandler{profiles: profiles}
}

func (h *ProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		h.writeJSON(w, http.StatusUnauthorized, profileAPIResponse{Error: http.StatusText(http.StatusUnauthorized)})
		return
	}
	if r.URL.Path == "/api/profile" {
		h.serveOwner(w, r, user.ID)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.writeJSON(w, http.StatusMethodNotAllowed, profileAPIResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
	profileID, err := parsePublicProfileID(r.URL.Path)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, profileAPIResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}
	profile, err := h.profiles.Get(profileID)
	if errors.Is(err, repository.ErrUserNotFound) {
		h.writeJSON(w, http.StatusNotFound, profileAPIResponse{Error: http.StatusText(http.StatusNotFound)})
		return
	}
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, profileAPIResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}
	h.writeJSON(w, http.StatusOK, profileAPIResponse{Profile: profileResponse(profile)})
}

func (h *ProfileHandler) serveOwner(w http.ResponseWriter, r *http.Request, ownerID int64) {
	switch r.Method {
	case http.MethodGet:
		profile, err := h.profiles.Get(ownerID)
		if err != nil {
			h.writeError(w, err)
			return
		}
		h.writeJSON(w, http.StatusOK, profileAPIResponse{Profile: profileResponse(profile)})
	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			h.writeJSON(w, http.StatusBadRequest, profileAPIResponse{Error: "invalid form"})
			return
		}
		age, err := strconv.Atoi(r.FormValue("age"))
		if err != nil {
			h.writeJSON(w, http.StatusBadRequest, profileAPIResponse{Error: validation.ErrInvalidProfile.Error()})
			return
		}
		profile, err := h.profiles.Update(ownerID, model.ProfileUpdate{
			FirstName: r.FormValue("first_name"), LastName: r.FormValue("last_name"),
			Age: age, Gender: r.FormValue("gender"),
		})
		if errors.Is(err, validation.ErrInvalidProfile) {
			h.writeJSON(w, http.StatusBadRequest, profileAPIResponse{Error: "profile fields are invalid"})
			return
		}
		if err != nil {
			h.writeError(w, err)
			return
		}
		h.writeJSON(w, http.StatusOK, profileAPIResponse{Profile: profileResponse(profile)})
	default:
		w.Header().Set("Allow", "GET, PUT")
		h.writeJSON(w, http.StatusMethodNotAllowed, profileAPIResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
	}
}

func (h *ProfileHandler) writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrUserNotFound) {
		h.writeJSON(w, http.StatusNotFound, profileAPIResponse{Error: http.StatusText(http.StatusNotFound)})
		return
	}
	h.writeJSON(w, http.StatusInternalServerError, profileAPIResponse{Error: http.StatusText(http.StatusInternalServerError)})
}

func profileResponse(profile model.Profile) *publicProfileResponse {
	return &publicProfileResponse{
		ID: profile.ID, Username: profile.Username, FirstName: profile.FirstName,
		LastName: profile.LastName, Age: profile.Age, Gender: profile.Gender,
	}
}

func parsePublicProfileID(path string) (int64, error) {
	const prefix, suffix = "/api/users/", "/profile"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, strconv.ErrSyntax
	}
	rawID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if rawID == "" || strings.Contains(rawID, "/") {
		return 0, strconv.ErrSyntax
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func (h *ProfileHandler) writeJSON(w http.ResponseWriter, status int, response profileAPIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
