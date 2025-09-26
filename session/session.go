package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Session represents an HTTP session with associated data and configuration.
// It tracks creation time, modification status, and whether the session has
// been destroyed.
type Session struct {
	id          string
	createdAt   time.Time
	values      map[string]any
	isDestroyed bool
	isModified  bool
}

// newSession creates a new Session with a unique ID, current timestamp,
// and an empty values map. This is used internally by the Manager.
func newSession() *Session {
	return &Session{
		id:        genSessionID(),
		createdAt: time.Now(),
		values:    make(map[string]any),
	}
}

// Destroy removes the session by clearing all values and marking it
// as destroyed and modified.
func (s *Session) Destroy() {
	s.Clear()
	s.isModified = true
	s.isDestroyed = true
}

// Set adds or updates a value in the session. Marks the session as modified.
func (s *Session) Set(key string, value interface{}) {
	s.isModified = true
	s.values[key] = value
}

// GetCreatedAt returns the time when the session was created.
func (s *Session) GetCreatedAt() time.Time {
	return s.createdAt
}

// GetID returns the session's unique identifier.
func (s *Session) GetID() string {
	return s.id
}

// Get retrieves a value from the session.
// Returns nil if the key doesn't exist.
func (s *Session) Get(key string) interface{} {
	return s.values[key]
}

// GetInt retrieves an int value from the session. Returns 0 if not found or
// type mismatch.
func (s *Session) GetInt(key string) int {
	v, _ := s.values[key].(int)
	return v
}

// GetUint retrieves a uint value from the session. Returns 0 if not found or
// type mismatch.
func (s *Session) GetUint(key string) uint {
	v, _ := s.values[key].(uint)
	return v
}

// GetBool retrieves a bool value from the session. Returns false if not found
// or type mismatch.
func (s *Session) GetBool(key string) bool {
	v, _ := s.values[key].(bool)
	return v
}

// GetFloat32 retrieves a float32 value from the session. Returns 0 if not found
// or type mismatch.
func (s *Session) GetFloat32(key string) float32 {
	v, _ := s.values[key].(float32)
	return v
}

// GetFloat64 retrieves a float64 value from the session. Returns 0 if not found
// or type mismatch.
func (s *Session) GetFloat64(key string) float64 {
	v, _ := s.values[key].(float64)
	return v
}

// GetString retrieves a string value from the session. Returns "" if not found
// or type mismatch.
func (s *Session) GetString(key string) string {
	v, _ := s.values[key].(string)
	return v
}

// Delete removes a value from the session and marks it as modified.
func (s *Session) Delete(key string) {
	s.isModified = true
	delete(s.values, key)
}

// Clear removes all values from the session.
func (s *Session) Clear() {
	s.isModified = true
	s.values = make(map[string]interface{})
}

// genSessionID generates a cryptographically random 16-byte session ID
// encoded as a hex string.
func genSessionID() string {
	id := make([]byte, 16)
	rand.Read(id)
	return hex.EncodeToString(id)
}
