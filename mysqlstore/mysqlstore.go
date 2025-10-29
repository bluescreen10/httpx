// Package mysqlstore provides an redis session storage implementation.
//
// MySQLStore allows storing, retrieving, and deleting session-like
// data keyed by a string token. Each record has an expiration time,
// and the store supports periodic cleanup of expired sessions.
package mysqlstore

import (
	"database/sql"
	"fmt"
	"time"
)

type MySQLStore struct {
	db *sql.DB
}

func New(db *sql.DB) (*MySQLStore, error) {
	err := createTable(db)
	return &MySQLStore{db: db}, err
}

// Get retrieves the data associated with the given token.Returns
// the data, a boolean indicating whether the token was found and
// not expired, and an error.
func (s *MySQLStore) Get(token string) ([]byte, bool, error) {

	stmt := "SELECT data FROM sessions WHERE token = ? AND UTC_TIMESTAMP(6) < expires_at"
	row := s.db.QueryRow(stmt, token)

	var data []byte
	err := row.Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}
	return data, true, nil
}

// Set stores the data under the given token with an expiration time. If
// a record with the same token already exists, it is overwritten. The
// expiresAt parameter specifies when the record should be considered expired.
func (s *MySQLStore) Set(token string, data []byte, expiresAt time.Time) error {
	stmt := "INSERT INTO sessions(token, data, expires_at) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE data = VALUES(data), expires_at = VALUES(expires_at)"
	_, err := s.db.Exec(stmt, token, data, expiresAt.UTC())
	return err
}

// Delete removes the data associated with the given token.
func (s *MySQLStore) Delete(token string) error {
	stmt := "DELETE FROM sessions WHERE token = ?"
	_, err := s.db.Exec(stmt, token)
	return err
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
func (s *MySQLStore) PeriodicCleanUp(interval time.Duration, stop <-chan (struct{})) {
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
func (s *MySQLStore) deleteExpired() {
	stmt := "DELETE FROM sessions WHERE  UTC_TIMESTAMP(6) > expires_at"
	s.db.Exec(stmt)
}

func createTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
			token CHAR(36) COLLATE utf8mb4_bin PRIMARY KEY,
			data BLOB NOT NULL,
			expires_at TIMESTAMP(6) NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions (expires_at)`)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}
