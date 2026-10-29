package service

import (
	"errors"

	"forum/internal/model"
	"forum/internal/validation"
)

var ErrProfileAuthenticationRequired = errors.New("profile access requires authentication")

type ProfileRepository interface {
	PublicProfileByID(userID int64) (model.Profile, error)
	UpdateProfile(userID int64, update model.ProfileUpdate) error
}

type ProfileService struct{ profiles ProfileRepository }

func NewProfileService(profiles ProfileRepository) *ProfileService {
	return &ProfileService{profiles: profiles}
}

func (s *ProfileService) Get(userID int64) (model.Profile, error) {
	if userID <= 0 {
		return model.Profile{}, ErrProfileAuthenticationRequired
	}
	return s.profiles.PublicProfileByID(userID)
}

func (s *ProfileService) Update(userID int64, update model.ProfileUpdate) (model.Profile, error) {
	if userID <= 0 {
		return model.Profile{}, ErrProfileAuthenticationRequired
	}
	validated, err := validation.ValidateProfile(update)
	if err != nil {
		return model.Profile{}, err
	}
	if err := s.profiles.UpdateProfile(userID, validated); err != nil {
		return model.Profile{}, err
	}
	return s.profiles.PublicProfileByID(userID)
}
