// Package redisstore provides an redis session storage implementation.
//
// RedisStore allows storing, retrieving, and deleting session-like
// data keyed by a string token. Each record has an expiration time,
// and the store supports periodic cleanup of expired sessions.
package redisstore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is an redis backed storage for session-like data.
type RedisStore struct {
	rdb *redis.Client
}

// New creates and returns a new RedisStore instance.
// If the sessions table doesn't exists it is created.
func New(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb}
}

// Get retrieves the data associated with the given token.Returns
// the data, a boolean indicating whether the token was found and
// not expired, and an error.
func (s *RedisStore) Get(token string) ([]byte, bool, error) {
	data, err := s.rdb.Get(context.Background(), token).Bytes()
	if err != nil {
		if err == redis.Nil {
			return []byte{}, false, nil
		} else {
			return []byte{}, false, err
		}
	}

	return data, true, nil
}

// Set stores the data under the given token with an expiration time. If
// a record with the same token already exists, it is overwritten. The
// expiresAt parameter specifies when the record should be considered expired.
func (s *RedisStore) Set(token string, data []byte, expiresAt time.Time) error {
	return s.rdb.Set(context.Background(), token, data, time.Until(expiresAt)).Err()
}

// Delete removes the data associated with the given token. If the token
// does not exist, this is a no-op.
func (s *RedisStore) Delete(token string) error {
	return s.rdb.Del(context.Background(), token).Err()
}
