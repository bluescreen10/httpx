// Package memstore provides an in-memory session storage implementation.
//
// Memstore allows storing, retrieving, and deleting session-like
// data keyed by a string token. Each record has an expiration time,
// and the store supports periodic cleanup of expired sessions.
//
// This package is suitable for single-process applications or testing
// scenarios. It is not persistent and does not share state across
// processes.
package memstore

import (
	"sync"
	"time"
)

// Memstore is an in-memory storage for session-like data.
// It is safe for concurrent use by multiple goroutines.
type Memstore struct {
	sessions sync.Map
}

// record represents a single stored session, containing the data
// and its expiration time.
type record struct {
	expiresAt time.Time
	data      []byte
}

// New creates and returns a new Memstore instance.
func New() *Memstore {
	return &Memstore{}
}

// Get retrieves the data associated with the given token.Returns
// the data, a boolean indicating whether the token was found and
// not expired, and an error. If the record has expired, it is
// automatically deleted and Get returns false.
func (m *Memstore) Get(token string) ([]byte, bool, error) {
	r, ok := m.sessions.Load(token)

	if !ok {
		return []byte{}, false, nil
	}

	rec := r.(record)
	if time.Now().After(rec.expiresAt) {
		m.Delete(token)
		return []byte{}, false, nil
	}

	return rec.data, true, nil
}

// Set stores the data under the given token with an expiration time. If
// a record with the same token already exists, it is overwritten. The
// expiresAt parameter specifies when the record should be considered expired.
func (m *Memstore) Set(token string, data []byte, expiresAt time.Time) error {
	rec := record{expiresAt: expiresAt, data: data}
	m.sessions.Store(token, rec)
	return nil
}

// Delete removes the data associated with the given token. If the token
// does not exist, this is a no-op.
func (m *Memstore) Delete(token string) error {
	m.sessions.Delete(token)
	return nil
}

// PeriodicCleanUp runs a loop that periodically deletes expired sessions.
// The cleanup runs every interval duration until a value is received on
// the stop channel, at which point the loop returns.
//
// Example usage:
//
//	stop := make(chan struct{})
//	go store.PeriodicCleanUp(time.Minute, stop)
//	...
//	close(stop) // stop the cleanup
func (m *Memstore) PeriodicCleanUp(interval time.Duration, stop <-chan (struct{})) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.deleteExpired()
		case <-stop:
			return
		}
	}
}

// deleteExpired removes all expired records from the Memstore.
func (m *Memstore) deleteExpired() {
	m.sessions.Range(func(key, value any) bool {
		rec := value.(record)
		if time.Now().After(rec.expiresAt) {
			m.Delete(key.(string))
		}
		return true
	})
}
