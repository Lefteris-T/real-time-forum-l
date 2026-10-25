// Package validation normalizes untrusted input and enforces size/content rules.
package validation

import (
	"fmt"
	"net/mail"
	"strings"
)

const (
	minUsernameLength = 3
	maxUsernameLength = 32
	minPasswordLength = 8
	maxPasswordLength = 72
	maxEmailLength    = 254
)

// FieldError identifies the field that failed validation.
type FieldError struct {
	Field   string
	Message string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func fieldError(field, message string) error {
	return FieldError{Field: field, Message: message}
}

// RegistrationInput is raw account-creation form data.
type RegistrationInput struct {
	Email     string
	Username  string
	Age       int
	Gender    string
	FirstName string
	LastName  string
	Password  string
}

// LoginInput is raw credential form data.
type LoginInput struct {
	Email    string
	Password string
}

// ValidateRegistration returns normalized email and username values while
// preserving the password exactly as submitted for bcrypt.
func ValidateRegistration(input RegistrationInput) (RegistrationInput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	username := strings.ToLower(strings.TrimSpace(input.Username))
	gender := strings.TrimSpace(input.Gender)
	firstName := strings.TrimSpace(input.FirstName)
	lastName := strings.TrimSpace(input.LastName)

	if email == "" {
		return RegistrationInput{}, fieldError("email", "is required")
	}
	if len(email) > maxEmailLength {
		return RegistrationInput{}, fieldError("email", "is too long")
	}

	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		return RegistrationInput{}, fieldError("email", "is invalid")
	}

	at := strings.LastIndexByte(email, '@')
	if at < 1 || !strings.Contains(email[at+1:], ".") {
		return RegistrationInput{}, fieldError("email", "domain must contain a dot")
	}

	if username == "" {
		return RegistrationInput{}, fieldError("username", "is required")
	}
	if len(username) < minUsernameLength {
		return RegistrationInput{}, fieldError(
			"username",
			fmt.Sprintf("must be at least %d characters", minUsernameLength),
		)
	}
	if len(username) > maxUsernameLength {
		return RegistrationInput{}, fieldError(
			"username",
			fmt.Sprintf("must be at most %d characters", maxUsernameLength),
		)
	}

	if input.Age < 1 || input.Age > 120 {
		return RegistrationInput{}, fieldError("age", "must be between 1 and 120")
	}
	if gender == "" {
		return RegistrationInput{}, fieldError("gender", "is required")
	}
	if firstName == "" {
		return RegistrationInput{}, fieldError("first_name", "is required")
	}
	if lastName == "" {
		return RegistrationInput{}, fieldError("last_name", "is required")
	}

	if input.Password == "" {
		return RegistrationInput{}, fieldError("password", "is required")
	}
	if len(input.Password) < minPasswordLength {
		return RegistrationInput{}, fieldError(
			"password",
			fmt.Sprintf("must be at least %d characters", minPasswordLength),
		)
	}
	if len(input.Password) > maxPasswordLength {
		return RegistrationInput{}, fieldError(
			"password",
			fmt.Sprintf("must be at most %d bytes", maxPasswordLength),
		)
	}

	return RegistrationInput{
		Email:     email,
		Username:  username,
		Age:       input.Age,
		Gender:    gender,
		FirstName: firstName,
		LastName:  lastName,
		Password:  input.Password,
	}, nil
}

// ValidateLogin normalizes the email and applies safe input-size limits.
func ValidateLogin(input LoginInput) (LoginInput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	if email == "" {
		return LoginInput{}, fieldError("email", "is required")
	}
	if len(email) > maxEmailLength {
		return LoginInput{}, fieldError("email", "is too long")
	}

	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		return LoginInput{}, fieldError("email", "is invalid")
	}

	at := strings.LastIndexByte(email, '@')
	if at < 1 || !strings.Contains(email[at+1:], ".") {
		return LoginInput{}, fieldError("email", "domain must contain a dot")
	}

	if input.Password == "" {
		return LoginInput{}, fieldError("password", "is required")
	}
	if len(input.Password) > maxPasswordLength {
		return LoginInput{}, fieldError(
			"password",
			fmt.Sprintf("must be at most %d bytes", maxPasswordLength),
		)
	}

	return LoginInput{
		Email:    email,
		Password: input.Password,
	}, nil
}
