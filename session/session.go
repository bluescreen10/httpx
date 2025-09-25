// Package session provides HTTP session management functionality with pluggable storage backends.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Session represents an HTTP session with associated data and configuration.
type Session struct {
	// Unique identifier for this session
	id string

	// used to determine the session duration
	createdAt time.Time

	// Session data as key-value pairs
	values map[string]any

	// Indicates if the session needs to be destroyed
	isDestroyed bool

	isModified bool
}

// NewSession creates a new session with the specified name and store.
// The session is initialized with default options and an empty values map.
func newSession() *Session {
	return &Session{
		id:        genSessionID(),
		createdAt: time.Now(),
		values:    make(map[string]any),
	}
}

// Destroy removes the session
func (s *Session) Destroy() {
	s.Clear()
	s.isModified = true
	s.isDestroyed = true
}

// Set adds or updates a value in the session.
func (s *Session) Set(key string, value interface{}) {
	s.isModified = true
	s.values[key] = value
}

func (s *Session) GetCreatedAt() time.Time {
	return s.createdAt
}

func (s *Session) GetID() string {
	return s.id
}

// Get retrieves a value from the session.
// Returns nil if the key doesn't exist.
func (s *Session) Get(key string) interface{} {
	return s.values[key]
}

func (s *Session) GetInt(key string) int {
	v, _ := s.values[key].(int)
	return v
}

func (s *Session) GetUint(key string) uint {
	v, _ := s.values[key].(uint)
	return v
}

func (s *Session) GetBool(key string) bool {
	v, _ := s.values[key].(bool)
	return v
}

func (s *Session) GetFloat32(key string) float32 {
	v, _ := s.values[key].(float32)
	return v
}

func (s *Session) GetFloat64(key string) float64 {
	v, _ := s.values[key].(float64)
	return v
}

func (s *Session) GetString(key string) string {
	v, _ := s.values[key].(string)
	return v
}

// Delete removes a value from the session.
func (s *Session) Delete(key string) {
	s.isModified = true
	delete(s.values, key)
}

// Clear removes all values from the session.
func (s *Session) Clear() {
	s.isModified = true
	s.values = make(map[string]interface{})
}

func genSessionID() string {
	id := make([]byte, 16)
	rand.Read(id)
	return hex.EncodeToString(id)
}
