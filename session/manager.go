// Package session provides a middleware-based session management system
// for HTTP servers in Go. It supports cookie-based sessions, idle timeouts,
// configurable persistence, and pluggable serialization codecs.
// designed heavily inspired by: https://github.com/alexedwards/scs
package session

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Options contains configuration for sessions, including cookie parameters,
// lifetime, idle timeout, and serialization codec.
type Options struct {
	// LifeTime is the maximum duration for which the session is valid. (default 24hr)
	LifeTime time.Duration

	// IdleTimeout is the maximum duration of inactivity before a session expires. (default 0 -> no timeout)
	IdleTimeout time.Duration

	// Codec is responsible for encoding and decoding session data.
	Codec Codec

	// Name sets the cookie Name
	Name string

	// Path restricts the cookie to a specific path
	Path string

	// Domain restricts the cookie to a specific domain
	Domain string

	// Secure indicates the cookie should only be sent over HTTPS
	Secure bool

	// HttpOnly prevents client-side JavaScript access to the cookie
	HttpOnly bool

	// Partitioned indicates the cookie is partitioned (Chrome's CHIPS)
	Partitioned bool

	// SameSite controls when cookies are sent with cross-site requests
	SameSite http.SameSite

	// Persisted determines if the cookie should be persisted or destroyed
	// when the user closes the browser
	Persisted bool
}

// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey int

var currentKey contextKey = 0
var mu sync.Mutex

// sessionResponseWriter wraps http.ResponseWriter to intercept writes
// and ensure the session is saved before any headers or body are written.
type sessionResponseWriter struct {
	http.ResponseWriter
	mngr      *Manager
	sess      *Session
	isWritten bool
}

// Write saves the session before writing the response body if it hasn't
// already been saved.
func (w *sessionResponseWriter) Write(b []byte) (int, error) {
	if !w.isWritten {
		w.isWritten = true
		w.mngr.save(w.ResponseWriter, w.sess)
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader saves the session before writing the response headers
// if it hasn't already been saved.
func (w *sessionResponseWriter) WriteHeader(statusCode int) {
	if !w.isWritten {
		w.isWritten = true
		w.mngr.save(w.ResponseWriter, w.sess)
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// Example demonstrates creating a Manager and using the LoadAndSave middleware.
//
//	package main
//
//	import (
//	    "fmt"
//	    "net/http"
//	    "time"
//
//	    "github.com/bluescreen10/httpx/session"
//	    "github.com/bluescreen10/httpx/store/memstore"
//	)
//
//	func main() {
//	    store := memstore.New()
//	    mgr := session.NewManager(store, session.Options{
//	        Name:     "my_session",
//	        LifeTime: 2 * time.Hour,
//	    })
//
//	    mux := http.NewServeMux()
//	    mux.Handle("/", mgr.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
// Manager manages HTTP sessions using a Store backend and session Options.
type Manager struct {
	store Store
	opts  Options
	key   contextKey
}

// LoadAndSave is an HTTP middleware that automatically loads a session from
// the request cookie and saves it back to the store after the handler executes.
func (m *Manager) LoadAndSave(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		var token string
		cookie, err := r.Cookie(m.opts.Name)
		if err == nil {
			token = cookie.Value
		}
		sess, err := m.load(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sr := r.WithContext(context.WithValue(r.Context(), m.key, sess))
		sw := &sessionResponseWriter{w, m, sess, false}
		next.ServeHTTP(sw, sr)

		if !sw.isWritten {
			m.save(w, sess)
		}
	})
}

// Get retrieves the current session from the request context.
// It always returns a valid session object, never nil.
func (m *Manager) Get(r *http.Request) *Session {
	return r.Context().Value(m.key).(*Session)
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

	createdAt, values, err := m.opts.Codec.Decode(data)
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

	expiresAt := sess.createdAt.Add(m.opts.LifeTime)

	if sess.isModified {
		data, err := m.opts.Codec.Encode(sess.createdAt, sess.values)
		if err != nil {
			return err
		}
		err = m.store.Set(sess.id, data, expiresAt)
		if err != nil {
			return err
		}
	}

	if m.opts.IdleTimeout > 0 {
		idleExpires := time.Now().Add(m.opts.IdleTimeout)
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
		Name:        m.opts.Name,
		Domain:      m.opts.Domain,
		HttpOnly:    m.opts.HttpOnly,
		Path:        m.opts.Path,
		SameSite:    m.opts.SameSite,
		Secure:      m.opts.Secure,
		Partitioned: m.opts.Partitioned,
	}

	if expiresAt.IsZero() {
		cookie.Expires = time.Unix(1, 0)
		cookie.MaxAge = -1
	} else if m.opts.Persisted {
		cookie.Expires = time.Unix(expiresAt.Unix()+1, 0)
		cookie.MaxAge = int(time.Until(expiresAt).Seconds() + 1)
	}

	http.SetCookie(w, cookie)
}

// NewManager creates a new session Manager with the given store and optional
// session Options.
func NewManager(store Store, opts ...Options) *Manager {
	mngr := new(Manager)
	mngr.store = store
	mngr.opts = defaultOptions

	// obtain a context key
	mu.Lock()
	mngr.key = currentKey
	currentKey++
	mu.Unlock()

	for _, o := range opts {
		if o.Domain != "" {
			mngr.opts.Domain = o.Domain
		}

		if o.Path != "" {
			mngr.opts.Path = o.Path
		}

		if o.Name != "" {
			mngr.opts.Name = o.Name
		}

		if o.LifeTime > 0 {
			mngr.opts.LifeTime = o.LifeTime
		}

		if o.SameSite > 0 {
			mngr.opts.SameSite = o.SameSite
		}

		if o.Codec != nil {
			mngr.opts.Codec = o.Codec
		}

		mngr.opts.HttpOnly = o.HttpOnly
		mngr.opts.Partitioned = o.Partitioned
		mngr.opts.Secure = o.Secure
		mngr.opts.IdleTimeout = o.IdleTimeout
		mngr.opts.Persisted = o.Persisted
	}

	return mngr
}

var defaultOptions = Options{
	LifeTime:  24 * time.Hour,
	Codec:     gobCodec{},
	Name:      "session_id",
	Path:      "/",
	HttpOnly:  true,
	SameSite:  http.SameSiteLaxMode,
	Persisted: true,
}
