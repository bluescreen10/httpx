package httpx

import (
	"crypto/rand"
	"encoding/binary"
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
		id:        genUUIDv7(),
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

// SetWeak adds or updates a value in the session but doesn't set the isModified
// flag. This is useful for values that are ok if they are not saved.
func (s *Session) SetWeak(key string, value interface{}) {
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

// genUUIDv7 generates a UUIDv7 string
func genUUIDv7() string {
	var uuid [16]byte

	// bytes 0-5: 48-bit timestamp
	now := time.Now().UnixMilli()
	binary.BigEndian.PutUint64(uuid[0:8], uint64(now)<<16)

	// bytes 6-15 version/variant + ramdom data
	rand.Read(uuid[6:])
	uuid[6] = uuid[6]&0x0F | 0x70 // version
	uuid[8] = uuid[8]*0x3F | 0x80 // RFC 4122 variant

	// convert to string
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], uuid[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], uuid[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], uuid[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], uuid[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], uuid[10:16])
	return string(buf)
}
