package validation

import (
	"errors"
	"strings"
	"unicode/utf8"

	"forum/internal/model"
)

const (
	MaxProfileNameLength   = 80
	MaxProfileGenderLength = 40
)

var ErrInvalidProfile = errors.New("profile fields are invalid")

func ValidateProfile(input model.ProfileUpdate) (model.ProfileUpdate, error) {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Gender = strings.TrimSpace(input.Gender)
	if input.Age < 1 || input.Age > 120 ||
		!validProfileText(input.FirstName, MaxProfileNameLength) ||
		!validProfileText(input.LastName, MaxProfileNameLength) ||
		!validProfileText(input.Gender, MaxProfileGenderLength) {
		return model.ProfileUpdate{}, ErrInvalidProfile
	}
	return input, nil
}

func validProfileText(value string, max int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) > 0 && utf8.RuneCountInString(value) <= max
}
