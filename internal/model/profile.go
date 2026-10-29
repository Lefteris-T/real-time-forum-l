package model

// Profile contains only fields approved for public profile responses.
type Profile struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	Age       int
	Gender    string
}

// ProfileUpdate contains the editable public fields. Username is the stable
// account identifier and cannot be changed through profile editing.
type ProfileUpdate struct {
	FirstName string
	LastName  string
	Age       int
	Gender    string
}
