package validation

import (
	"testing"

	"forum/internal/model"
)

func TestValidateProfileTrimsAndChecksPublicFields(t *testing.T) {
	got, err := ValidateProfile(model.ProfileUpdate{FirstName: " New ", LastName: " Name ", Age: 32, Gender: " other "})
	if err != nil || got.FirstName != "New" || got.LastName != "Name" || got.Gender != "other" {
		t.Fatalf("ValidateProfile() = %+v, error %v", got, err)
	}
	for _, invalid := range []model.ProfileUpdate{
		{FirstName: "", LastName: "Name", Age: 32, Gender: "other"},
		{FirstName: "First", LastName: "Name", Age: 0, Gender: "other"},
		{FirstName: "First", LastName: "Name", Age: 32, Gender: ""},
		{FirstName: string(make([]rune, MaxProfileNameLength+1)), LastName: "Name", Age: 32, Gender: "other"},
	} {
		if _, err := ValidateProfile(invalid); err != ErrInvalidProfile {
			t.Errorf("ValidateProfile(%+v) error = %v, want invalid profile", invalid, err)
		}
	}
}
