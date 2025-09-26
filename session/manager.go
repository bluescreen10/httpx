// Package session provides a middleware-based session management system
// for HTTP servers in Go. It supports cookie-based sessions, idle timeouts,
// configurable persistence, and pluggable serialization codecs.
// Designed heavily inspired by: https://github.com/alexedwards/scs
//
// Usage:
//
//	package main
//
//	import (
//	    "fmt"
//	    "net/http"
//	    "time"
//
//	    "github.com/bluescreen10/httpx/session"
//	    "github.com/bluescreen10/httpx/memstore"
//	)
//
//	func main() {
//	    store := memstore.New()
//	    mgr := session.NewManager(store, session.Options{
//	        Name:     "my_session",
//	        Lifetime: 2 * time.Hour,
//	    })
//
//	    mux := http.NewServeMux()
//	    mux.Handle("/", mgr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        sess := mgr.Get(r)
//	        count := sess.GetInt("count")
//	        count++
//	        sess.Set("count", count)
//	        fmt.Fprintf(w, "You have visited %d times\n", count)
//	    })))
//
//	    http.ListenAndServe(":8080", mux)
//	}
//
// designed heavily inspired by: https://github.com/alexedwards/scs
package session

import (
	"context"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to intercept writes
// and ensure the session is saved before any headers or body are written.
type responseWriter struct {
	http.ResponseWriter
	mngr      *Manager
	sess      *Session
	isWritten bool
}

// Write saves the session before writing the response body if it hasn't
// already been saved.
func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.isWritten {
		w.isWritten = true
		w.mngr.save(w.ResponseWriter, w.sess)
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader saves the session before writing the response headers
// if it hasn't already been saved.
func (w *responseWriter) WriteHeader(statusCode int) {
	if !w.isWritten {
		w.isWritten = true
		w.mngr.save(w.ResponseWriter, w.sess)
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// Middleware-based session management stores session data in a Store backend,
// manages cookies, handles idle timeouts, and provides a load/save workflow
// automatically via the Handler middleware interface.

// Manager manages HTTP sessions using a Store backend and session options.
type Manager struct {
	store             Store
	lifetime          time.Duration
	idleTimeout       time.Duration
	codec             Codec
	cookieName        string
	cookiePath        string
	cookieDomain      string
	cookieSecure      bool
	cookieHttpOnly    bool
	cookiePartitioned bool
	cookieSameSite    http.SameSite
	cookiePersisted   bool
	key               *struct{}
}

type config func(*Manager)

// WithLifetime sets the lifetime of the session. (default 24hr.)
func WithLifetime(lifetime time.Duration) config {
	return config(func(m *Manager) {
		m.lifetime = lifetime
	})
}

// WithIdleTimeout sets the idle timeout for the session. (default no timeout.)
func WithIdleTimeout(timeout time.Duration) config {
	return config(func(m *Manager) {
		m.idleTimeout = timeout
	})
}

// WithName sets the cookie name for the session. (default "session_id".)
func WithName(name string) config {
	return config(func(m *Manager) {
		m.cookieName = name
	})
}

// WithPath sets the cookie path. (default "/".)
func WithPath(Path string) config {
	return config(func(m *Manager) {
		m.cookiePath = Path
	})
}

// WithDomain sets the cookie domain. (default "".)
func WithDomain(Domain string) config {
	return config(func(m *Manager) {
		m.cookieDomain = Domain
	})
}

// WithSecure sets the Secure flag on the cookie. (default false)
func WithSecure(secure bool) config {
	return config(func(m *Manager) {
		m.cookieSecure = secure
	})
}

// WithHttpOnly sets the HttpOnly flag on the cookie. (default true)
func WithHttpOnly(httpOnly bool) config {
	return config(func(m *Manager) {
		m.cookieHttpOnly = httpOnly
	})
}

// WithPartitioned sets the Partitioned flag on the cookie. (default false)
func WithPartitioned(partitioned bool) config {
	return config(func(m *Manager) {
		m.cookiePartitioned = partitioned
	})
}

// WithSameSite sets the SameSite policy for the cookie. (default Lax)
func WithSameSite(sameSite http.SameSite) config {
	return config(func(m *Manager) {
		m.cookieSameSite = sameSite
	})
}

// WithPersisted sets whether the cookie is persisted. (default true)
func WithPersisted(persisted bool) config {
	return config(func(m *Manager) {
		m.cookiePersisted = persisted
	})
}

// Handler wraps an http.Handler and provides load-and-save session functionality.
// It ensures that the session is loaded from the store and saved after the request.
func (m *Manager) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		var token string
		cookie, err := r.Cookie(m.cookieName)
		if err == nil {
			token = cookie.Value
		}
		sess, err := m.load(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sr := r.WithContext(context.WithValue(r.Context(), m.key, sess))
		sw := &responseWriter{w, m, sess, false}
		next.ServeHTTP(sw, sr)

		if !sw.isWritten {
			m.save(w, sess)
		}
	})
}

// Get retrieves the current session from the request context. It always
// returns a valid session object, never nil.
func (m *Manager) Get(r *http.Request) *Session {
	sess, ok := r.Context().Value(m.key).(*Session)
	if !ok {
		return newSession()
	}
	return sess
}

// load retrieves a session from the store by token. If the token is empty
// or the session is not found, a new session is created.
func (m *Manager) load(token string) (*Session, error) {

	if token == "" {
		return newSession(), nil
	}

	data, found, err := m.store.Get(token)
	if err != nil {
		return nil, err
	}

	if !found {
		return newSession(), nil
	}

	createdAt, values, err := m.codec.Decode(data)
	if err != nil {
		return nil, err
	}

	return &Session{id: token, createdAt: createdAt, values: values}, nil
}

// save persists the session to the store and updates the HTTP cookie.
// Destroyed sessions are deleted from the store and expired cookies are set.
func (m *Manager) save(w http.ResponseWriter, sess *Session) error {
	if sess.isDestroyed {
		err := m.store.Delete(sess.id)
		if err != nil {
			return err
		}
		m.writeCookie(w, sess.id, time.Time{})
		return nil
	}

	expiresAt := sess.createdAt.Add(m.lifetime)

	if sess.isModified {
		data, err := m.codec.Encode(sess.createdAt, sess.values)
		if err != nil {
			return err
		}
		err = m.store.Set(sess.id, data, expiresAt)
		if err != nil {
			return err
		}
	}

	if m.idleTimeout > 0 {
		idleExpires := time.Now().Add(m.idleTimeout)
		if idleExpires.Before(expiresAt) {
			expiresAt = idleExpires
		}
	}
	m.writeCookie(w, sess.id, expiresAt)
	return nil
}

// writeCookie sets or expires the session cookie on the HTTP response.
func (m *Manager) writeCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	cookie := &http.Cookie{
		Value:       token,
		Name:        m.cookieName,
		Domain:      m.cookieDomain,
		HttpOnly:    m.cookieHttpOnly,
		Path:        m.cookiePath,
		SameSite:    m.cookieSameSite,
		Secure:      m.cookieSecure,
		Partitioned: m.cookiePartitioned,
	}

	if expiresAt.IsZero() {
		cookie.Expires = time.Unix(1, 0)
		cookie.MaxAge = -1
	} else if m.cookiePersisted {
		cookie.Expires = time.Unix(expiresAt.Unix()+1, 0)
		cookie.MaxAge = int(time.Until(expiresAt).Seconds() + 1)
	}

	http.SetCookie(w, cookie)
}

// NewManager creates a new session Manager with a Store and optional configuration.
func NewManager(store Store, cfgs ...config) *Manager {
	mngr := &Manager{
		lifetime:        24 * time.Hour,
		codec:           gobCodec{},
		cookieName:      "session_id",
		cookiePath:      "/",
		cookieHttpOnly:  true,
		cookieSameSite:  http.SameSiteLaxMode,
		cookiePersisted: true,
		store:           store,
	}

	for _, cfg := range cfgs {
		cfg(mngr)
	}

	return mngr
}
