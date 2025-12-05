/*
Package user contains core data structures and logic related to user identity and session.

It defines the basic representation of a user within the chat system (the User struct),
used for passing user information both internally and to clients.
*/
package user

// User represents the basic identity information of a chat participant.
// Fields use JSON tags for serialization in WebSocket messages.
type User struct {

	// ID is the unique identifier for the user, typically a client-generated Guest ID.
	ID string `json:"id"`

	// Nickname is the display name of the user in the chat room.
	Nickname string `json:"nickname"`

	// Avatar is the URL for the user's avatar (currently unimplemented but reserved).
	Avatar string `json:"avatar,omitempty"`

	// UserType defines the role/status of the participant (e.g., "guest", "registered").
	UserType string `json:"userType"`
}
