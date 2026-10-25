package validation

import (
	"strings"
	"testing"
)

func TestValidateRegistrationValidInput(t *testing.T) {
	input := RegistrationInput{
		Email:    "  Lefteris@Example.COM  ",
		Username: "  lefteris  ",
		Age:      30, Gender: " test ", FirstName: " Test ", LastName: " User ",
		Password: "strong-password-123",
	}

	got, err := ValidateRegistration(input)
	if err != nil {
		t.Fatalf("ValidateRegistration() error: %v", err)
	}

	if got.Email != "lefteris@example.com" {
		t.Errorf(
			"Email = %q, want %q",
			got.Email,
			"lefteris@example.com",
		)
	}

	if got.Username != "lefteris" {
		t.Errorf(
			"Username = %q, want %q",
			got.Username,
			"lefteris",
		)
	}
	if got.Age != 30 || got.Gender != "test" || got.FirstName != "Test" || got.LastName != "User" {
		t.Fatalf("profile = %+v, want normalized submitted values", got)
	}

	if got.Password != "strong-password-123" {
		t.Errorf(
			"Password = %q, want original password",
			got.Password,
		)
	}
}
func TestValidateRegistrationRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input RegistrationInput
	}{
		{
			name: "blank email",
			input: RegistrationInput{
				Email:    "",
				Username: "lefteris",
				Password: "strong-password-123",
			},
		},
		{
			name: "whitespace email",
			input: RegistrationInput{
				Email:    "   ",
				Username: "lefteris",
				Password: "strong-password-123",
			},
		},
		{
			name: "malformed email",
			input: RegistrationInput{
				Email:    "not-an-email",
				Username: "lefteris",
				Password: "strong-password-123",
			},
		},
		{
			name: "blank username",
			input: RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "",
				Password: "strong-password-123",
			},
		},
		{
			name: "whitespace username",
			input: RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "   ",
				Password: "strong-password-123",
			},
		},
		{
			name: "username too short",
			input: RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "ab",
				Password: "strong-password-123",
			},
		},
		{
			name: "username too long",
			input: RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "abcdefghijklmnopqrstuvwxyz1234567",
				Password: "strong-password-123",
			},
		},
		{
			name: "blank password",
			input: RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "lefteris",
				Password: "",
			},
		},
		{
			name: "password too short",
			input: RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "lefteris",
				Password: "12345",
			},
		},
		{
			name: "password too long",
			input: RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "lefteris",
				Password: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateRegistration(tt.input)

			if err == nil {
				t.Fatal("ValidateRegistration() error = nil, want error")
			}
		})
	}
}
func TestValidateLoginValidInput(t *testing.T) {
	input := LoginInput{
		Email:    "  Lefteris@Example.COM  ",
		Password: "strong-password-123",
	}

	got, err := ValidateLogin(input)
	if err != nil {
		t.Fatalf("ValidateLogin() error: %v", err)
	}

	if got.Email != "lefteris@example.com" {
		t.Errorf(
			"Email = %q, want %q",
			got.Email,
			"lefteris@example.com",
		)
	}

	if got.Password != "strong-password-123" {
		t.Errorf(
			"Password = %q, want original password",
			got.Password,
		)
	}
}
func TestValidateLoginRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input LoginInput
	}{
		{
			name: "blank email",
			input: LoginInput{
				Email:    "",
				Password: "strong-password-123",
			},
		},
		{
			name: "whitespace email",
			input: LoginInput{
				Email:    "   ",
				Password: "strong-password-123",
			},
		},
		{
			name: "malformed email",
			input: LoginInput{
				Email:    "not-an-email",
				Password: "strong-password-123",
			},
		},
		{
			name: "blank password",
			input: LoginInput{
				Email:    "lefteris@example.com",
				Password: "",
			},
		},
		{
			name: "oversized email",
			input: LoginInput{
				Email:    strings.Repeat("a", 300) + "@example.com",
				Password: "strong-password-123",
			},
		},
		{
			name: "oversized password",
			input: LoginInput{
				Email:    "lefteris@example.com",
				Password: strings.Repeat("a", 73),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateLogin(tt.input)

			if err == nil {
				t.Fatal("ValidateLogin() error = nil, want error")
			}
		})
	}
}
