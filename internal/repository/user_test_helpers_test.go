package repository

import "forum/internal/model"

// insertTestUser supplies a complete, deterministic profile for tests whose
// behavior is unrelated to account profile data.
func insertTestUser(
	repo *UserRepository,
	email string,
	username string,
	passwordHash string,
) (int64, error) {
	return repo.Create(model.User{
		Email:        email,
		Username:     username,
		Age:          30,
		Gender:       "test",
		FirstName:    "Test",
		LastName:     "User",
		PasswordHash: passwordHash,
	})
}
