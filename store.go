package httpx

import "time"

// Store defines the interface for session storage backends.
// A Store is responsible for persisting and retrieving session data
// by a unique session token. Implementations may store sessions in
// memory, databases, caches, or any other durable storage system.
type Store interface {
	// Get retrieves the session data associated with the given token.
	// It returns the raw session data, a boolean indicating whether
	// the session was found, and an error if the lookup failed.
	Get(token string) (data []byte, found bool, err error)

	// Set stores the session data for the given token until the
	// specified expiration time. If a session with the same token
	// already exists, it should be overwritten.
	Set(token string, data []byte, expiresAt time.Time) error

	// Delete removes the session associated with the given token.
	// It returns an error if the deletion fails, but should not
	// return an error if the session does not exist.
	Delete(token string) error
}
