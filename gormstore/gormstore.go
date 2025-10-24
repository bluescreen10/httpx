// Package gormstore provides an gorm session storage implementation.
//
// GORMStore allows storing, retrieving, and deleting session-like
// data keyed by a string token. Each record has an expiration time,
// and the store supports periodic cleanup of expired sessions.
package gormstore

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// GORMStore is an gorm backed storage for session-like data.
type GORMStore struct {
	db *gorm.DB
}

// session represents a single stored session, containing the data
// and its expiration time.
type session struct {
	Token     string `gorm:"primaryKey;type:char(36)"`
	Data      []byte
	ExpiresAt time.Time `gorm:"index"`
}

// New creates and returns a new GORMStore instance.
// If the sessions table doesn't exists it is created.
func New(db *gorm.DB) (*GORMStore, error) {
	s := &GORMStore{db: db}
	return s, db.AutoMigrate(&session{})
}

// Get retrieves the data associated with the given token.Returns
// the data, a boolean indicating whether the token was found and
// not expired, and an error.
func (s *GORMStore) Get(token string) ([]byte, bool, error) {
	sess := &session{}
	tx := s.db.Where("token = ? AND expires_at >= ?", token, time.Now()).Limit(1).Find(sess)
	if tx.Error != nil || tx.RowsAffected == 0 {
		return nil, false, tx.Error
	}

	return sess.Data, true, nil
}

// Set stores the data under the given token with an expiration time. If
// a record with the same token already exists, it is overwritten. The
// expiresAt parameter specifies when the record should be considered expired.
func (s *GORMStore) Set(token string, data []byte, expiresAt time.Time) error {
	sess := &session{}
	tx := s.db.Where(session{Token: token}).Assign(session{Data: data, ExpiresAt: expiresAt}).FirstOrCreate(sess)
	return tx.Error
}

// Delete removes the data associated with the given token.
func (s *GORMStore) Delete(token string) error {
	tx := s.db.Delete(&session{}, "token = ?", token)
	return tx.Error
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
func (s *GORMStore) PeriodicCleanUp(interval time.Duration, stop <-chan (struct{})) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.deleteExpired()
		case <-stop:
			return
		}
	}
}

// deleteExpired removes all expired records from the Memstore.
func (s *GORMStore) deleteExpired() {
	tx := s.db.Delete(&session{}, "expires_at < ?", time.Now())
	if tx.Error != nil {
		log.Print(tx.Error)
	}
}
