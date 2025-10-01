// SessionManager provides a middleware-based session management system
// for HTTP servers in Go. It supports cookie-based sessions, idle timeouts,
// configurable persistence, and pluggable serialization codecs.
// Designed heavily inspired by: https://github.com/alexedwards/scs
//
// Usage:
//
//		package main
//
//		import (
//		    "fmt"
//		    "net/http"
//		    "time"
//
//		    "github.com/bluescreen10/httpx"
//		    "github.com/bluescreen10/httpx/memstore"
//		)
//
//		func main() {
//		    store := memstore.New()
//		    mgr := httpx.NewSessionManager(store)
//	        mgr.SetIdleTimeout(10 * time.Minute)
//
//		    mux := http.NewServeMux()
//		    mux.Handle("/", mgr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		        sess := mgr.Get(r)
//		        count := sess.GetInt("count")
//		        count++
//		        sess.Set("count", count)
//		        fmt.Fprintf(w, "You have visited %d times\n", count)
//		    })))
//
//		    http.ListenAndServe(":8080", mux)
//		}
package httpx

import (
	"context"
	"net/http"
	"time"
)

type sessionResponseWriter struct {
	http.ResponseWriter
	mngr      *SessionManager
	sess      *Session
	isWritten bool
}

func (w *sessionResponseWriter) Write(b []byte) (int, error) {
	if !w.isWritten {
		w.isWritten = true
		w.mngr.Save(w.ResponseWriter, w.sess)
	}
	return w.ResponseWriter.Write(b)
}

func (w *sessionResponseWriter) WriteHeader(statusCode int) {
	if !w.isWritten {
		w.isWritten = true
		w.mngr.Save(w.ResponseWriter, w.sess)
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// SessionManager manages HTTP sessions using a Store backend and session options.
type SessionManager struct {
	store       Store
	lifetime    time.Duration
	idleTimeout time.Duration
	codec       Codec
	cookie      CookieConfig
	key         *struct{}
}

type CookieConfig struct {
	Name        string
	Path        string
	Domain      string
	Secure      bool
	HttpOnly    bool
	Partitioned bool
	SameSite    http.SameSite
	Persisted   bool
}

func (m *SessionManager) SetIdleTimeout(timeout time.Duration) {
	m.idleTimeout = timeout
}

func (m *SessionManager) SetLifetime(lifetime time.Duration) {
	m.lifetime = lifetime
}

func (m *SessionManager) SetCookieConfig(cfg CookieConfig) {
	m.cookie = cfg
}

// Handler method is a middleware that provides load-and-save session functionality.
// It ensures that the session is loaded from the store and saved after the request.
func (m *SessionManager) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		var token string
		cookie, err := r.Cookie(m.cookie.Name)
		if err == nil {
			token = cookie.Value
		}
		sess, err := m.Load(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sr := r.WithContext(context.WithValue(r.Context(), m.key, sess))
		sw := &sessionResponseWriter{w, m, sess, false}
		next.ServeHTTP(sw, sr)

		if !sw.isWritten {
			m.Save(w, sess)
		}
	})
}

// Get retrieves the current session from the request context. This
// should be used only when using the middleware (Handler method).
func (m *SessionManager) Get(r *http.Request) *Session {
	sess, ok := r.Context().Value(m.key).(*Session)
	if !ok {
		return newSession()
	}
	return sess
}

// Load retrieves a session from the store by token. If the token is empty
// or the session is not found, a new session is created.
func (m *SessionManager) Load(token string) (*Session, error) {

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

// Save persists the session to the store and updates the HTTP cookie.
// Destroyed sessions are deleted from the store and expired cookies are set.
func (m *SessionManager) Save(w http.ResponseWriter, sess *Session) error {
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
		sess.isModified = false
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

func (m *SessionManager) writeCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	cookie := &http.Cookie{
		Value:       token,
		Name:        m.cookie.Name,
		Domain:      m.cookie.Domain,
		HttpOnly:    m.cookie.HttpOnly,
		Path:        m.cookie.Path,
		SameSite:    m.cookie.SameSite,
		Secure:      m.cookie.Secure,
		Partitioned: m.cookie.Partitioned,
	}

	if expiresAt.IsZero() {
		cookie.Expires = time.Unix(1, 0)
		cookie.MaxAge = -1
	} else if m.cookie.Persisted {
		cookie.Expires = time.Unix(expiresAt.Unix()+1, 0)
		cookie.MaxAge = int(time.Until(expiresAt).Seconds() + 1)
	}

	http.SetCookie(w, cookie)
}

// NewSessionManager returns a middleware-based session management stores
// session data in a Store backend, manages cookies, handles idle timeouts,
// and provides a load/save workflow automatically via the Handler middleware
// interface.
func NewSessionManager(store Store) *SessionManager {
	mngr := &SessionManager{
		lifetime: 24 * time.Hour,
		codec:    GobCodec{},
		store:    store,
		cookie: CookieConfig{
			Name:      "session_id",
			Path:      "/",
			HttpOnly:  true,
			SameSite:  http.SameSiteLaxMode,
			Persisted: true,
		},
	}
	return mngr
}
