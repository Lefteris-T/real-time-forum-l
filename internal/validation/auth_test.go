package validation

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func validRegistrationInput() RegistrationInput {
	return RegistrationInput{
		Email:     "lefteris@example.com",
		Username:  "lefteris",
		Age:       30,
		Gender:    "test",
		FirstName: "Test",
		LastName:  "User",
		Password:  "strong-password-123",
	}
}

func TestValidateRegistrationNormalizesInput(t *testing.T) {
	input := RegistrationInput{
		Email:     "  Lefteris@Example.COM  ",
		Username:  "  Lefteris  ",
		Age:       30,
		Gender:    " test ",
		FirstName: " Test ",
		LastName:  " User ",
		Password:  " strong-password-123 ",
	}

	got, err := ValidateRegistration(input)
	if err != nil {
		t.Fatalf("ValidateRegistration() error: %v", err)
	}

	if got.Email != "lefteris@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "lefteris@example.com")
	}
	if got.Username != "lefteris" {
		t.Errorf("Username = %q, want %q", got.Username, "lefteris")
	}
	if got.Age != 30 || got.Gender != "test" ||
		got.FirstName != "Test" || got.LastName != "User" {
		t.Errorf("profile = %+v, want normalized profile fields", got)
	}
	if got.Password != input.Password {
		t.Errorf("Password = %q, want it preserved exactly", got.Password)
	}
}

func TestValidateRegistrationAcceptsAgeBoundaries(t *testing.T) {
	for _, age := range []int{1, 120} {
		t.Run(string(rune(age)), func(t *testing.T) {
			input := validRegistrationInput()
			input.Age = age

			if _, err := ValidateRegistration(input); err != nil {
				t.Errorf("ValidateRegistration(age=%d) error: %v", age, err)
			}
		})
	}
}

func TestValidateRegistrationRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		mutate func(*RegistrationInput)
	}{
		{
			name:  "blank email",
			field: "email",
			mutate: func(input *RegistrationInput) {
				input.Email = "  "
			},
		},
		{
			name:  "malformed email",
			field: "email",
			mutate: func(input *RegistrationInput) {
				input.Email = "not-an-email"
			},
		},
		{
			name:  "email with display name",
			field: "email",
			mutate: func(input *RegistrationInput) {
				input.Email = "Lefteris <lefteris@example.com>"
			},
		},
		{
			name:  "blank username",
			field: "username",
			mutate: func(input *RegistrationInput) {
				input.Username = "  "
			},
		},
		{
			name:  "username too short",
			field: "username",
			mutate: func(input *RegistrationInput) {
				input.Username = "ab"
			},
		},
		{
			name:  "username too long",
			field: "username",
			mutate: func(input *RegistrationInput) {
				input.Username = strings.Repeat("a", maxUsernameLength+1)
			},
		},
		{
			name:  "age below minimum",
			field: "age",
			mutate: func(input *RegistrationInput) {
				input.Age = 0
			},
		},
		{
			name:  "age above maximum",
			field: "age",
			mutate: func(input *RegistrationInput) {
				input.Age = 121
			},
		},
		{
			name:  "blank gender",
			field: "gender",
			mutate: func(input *RegistrationInput) {
				input.Gender = "  "
			},
		},
		{
			name:  "blank first name",
			field: "first_name",
			mutate: func(input *RegistrationInput) {
				input.FirstName = "  "
			},
		},
		{
			name:  "blank last name",
			field: "last_name",
			mutate: func(input *RegistrationInput) {
				input.LastName = "  "
			},
		},
		{
			name:  "blank password",
			field: "password",
			mutate: func(input *RegistrationInput) {
				input.Password = ""
			},
		},
		{
			name:  "password too short",
			field: "password",
			mutate: func(input *RegistrationInput) {
				input.Password = "short"
			},
		},
		{
			name:  "password too long",
			field: "password",
			mutate: func(input *RegistrationInput) {
				input.Password = strings.Repeat("a", maxPasswordLength+1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validRegistrationInput()
			tt.mutate(&input)

			_, err := ValidateRegistration(input)
			if err == nil {
				t.Fatal("ValidateRegistration() error = nil, want error")
			}

			var got FieldError
			if !errors.As(err, &got) {
				t.Fatalf("error type = %T, want FieldError", err)
			}
			if got.Field != tt.field {
				t.Errorf("error field = %q, want %q", got.Field, tt.field)
			}
		})
	}
}

func TestValidateLoginNormalizesEmailAndPreservesPassword(t *testing.T) {
	input := LoginInput{
		Identifier: "  Lefteris@Example.COM  ",
		Password:   "password with spaces",
	}

	got, err := ValidateLogin(input)
	if err != nil {
		t.Fatalf("ValidateLogin() error: %v", err)
	}
	if got.Identifier != "lefteris@example.com" {
		t.Errorf("Identifier = %q, want %q", got.Identifier, "lefteris@example.com")
	}
	if got.Password != input.Password {
		t.Errorf("Password = %q, want it preserved exactly", got.Password)
	}
}

func TestValidateLoginNormalizesNicknameAndPreservesPassword(t *testing.T) {
	input := LoginInput{
		Identifier: "  Lefteris  ",
		Password:   "password with spaces",
	}

	got, err := ValidateLogin(input)
	if err != nil {
		t.Fatalf("ValidateLogin() error: %v", err)
	}
	if got.Identifier != "lefteris" {
		t.Errorf("Identifier = %q, want %q", got.Identifier, "lefteris")
	}
	if got.Password != input.Password {
		t.Errorf("Password = %q, want it preserved exactly", got.Password)
	}
}

func TestValidateLoginAcceptsNicknameLengthBoundaries(t *testing.T) {
	for _, nickname := range []string{
		strings.Repeat("a", minUsernameLength),
		strings.Repeat("a", maxUsernameLength),
	} {
		t.Run(fmt.Sprintf("length_%d", len(nickname)), func(t *testing.T) {
			if _, err := ValidateLogin(LoginInput{
				Identifier: nickname,
				Password:   "strong-password-123",
			}); err != nil {
				t.Errorf("ValidateLogin(%q) error: %v", nickname, err)
			}
		})
	}
}

func TestValidateLoginRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		mutate func(*LoginInput)
	}{
		{
			name:  "blank identifier",
			field: "identifier",
			mutate: func(input *LoginInput) {
				input.Identifier = " "
			},
		},
		{
			name:  "malformed email",
			field: "identifier",
			mutate: func(input *LoginInput) {
				input.Identifier = "not-an@email"
			},
		},
		{
			name:  "oversized email",
			field: "identifier",
			mutate: func(input *LoginInput) {
				input.Identifier = strings.Repeat("a", 300) + "@example.com"
			},
		},
		{
			name:  "nickname too short",
			field: "identifier",
			mutate: func(input *LoginInput) {
				input.Identifier = "ab"
			},
		},
		{
			name:  "nickname too long",
			field: "identifier",
			mutate: func(input *LoginInput) {
				input.Identifier = strings.Repeat("a", maxUsernameLength+1)
			},
		},
		{
			name:  "blank password",
			field: "password",
			mutate: func(input *LoginInput) {
				input.Password = ""
			},
		},
		{
			name:  "oversized password",
			field: "password",
			mutate: func(input *LoginInput) {
				input.Password = strings.Repeat("a", maxPasswordLength+1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := LoginInput{
				Identifier: "lefteris@example.com",
				Password:   "strong-password-123",
			}
			tt.mutate(&input)

			_, err := ValidateLogin(input)
			if err == nil {
				t.Fatal("ValidateLogin() error = nil, want error")
			}

			var got FieldError
			if !errors.As(err, &got) {
				t.Fatalf("error type = %T, want FieldError", err)
			}
			if got.Field != tt.field {
				t.Errorf("error field = %q, want %q", got.Field, tt.field)
			}
		})
	}
}
