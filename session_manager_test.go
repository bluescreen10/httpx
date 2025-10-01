package httpx_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluescreen10/httpx"
)

type mockstore struct {
	get    func(string) ([]byte, bool, error)
	set    func(string, []byte, time.Time) error
	delete func(string) error
}

func (s *mockstore) Get(token string) ([]byte, bool, error) {
	return s.get(token)
}

func (s *mockstore) Set(token string, data []byte, expiresAt time.Time) error {
	return s.set(token, data, expiresAt)
}

func (s *mockstore) Delete(token string) error {
	return s.delete(token)
}

var _ httpx.Store = &mockstore{}

func TestCreateSession(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)

	expectedId := 123
	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, nil
	}

	var storedData []byte
	store.set = func(token string, data []byte, _ time.Time) error {
		storedData = data
		return nil
	}

	h1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		sess.Set("user_id", expectedId)
	})

	r1 := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	w1 := httptest.NewRecorder()

	h := sm.Handler(h1)
	h.ServeHTTP(w1, r1)

	store.get = func(string) ([]byte, bool, error) {
		return storedData, true, nil
	}

	h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		if id := sess.GetInt("user_id"); id != expectedId {
			t.Fatalf("expected value '%d' got '%d'", expectedId, id)
		}
	})

	r2 := httptest.NewRequest("GET", "/", &bytes.Buffer{})
	w2 := httptest.NewRecorder()
	cookie := w1.Result().Header.Get("Set-Cookie")

	r2.Header.Set("Cookie", cookie)
	h = sm.Handler(h2)
	h.ServeHTTP(w2, r2)
}

func TestCreateSessionWithCookie(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)

	expectedId := 123
	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, nil
	}

	var storedData []byte
	store.set = func(token string, data []byte, _ time.Time) error {
		storedData = data
		return nil
	}

	h1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		if u := sess.GetInt("user_id"); u != 0 {
			t.Fatalf("expected '0' session but got '%d'", u)
		}
		sess.Set("user_id", expectedId)
	})

	cookie := "session_id=abc123;"
	r1 := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	r1.Header.Set("Cookie", cookie)
	w1 := httptest.NewRecorder()

	h := sm.Handler(h1)
	h.ServeHTTP(w1, r1)

	store.get = func(string) ([]byte, bool, error) {
		return storedData, true, nil
	}

	h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		if id := sess.GetInt("user_id"); id != expectedId {
			t.Fatalf("expected value '%d' got '%d'", expectedId, id)
		}
	})

	r2 := httptest.NewRequest("GET", "/", &bytes.Buffer{})
	w2 := httptest.NewRecorder()
	r2.Header.Set("Cookie", cookie)

	h = sm.Handler(h2)
	h.ServeHTTP(w2, r2)
}

func TestErrorLoadingSession(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)

	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, errors.New("test")
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})

	cookie := "session_id=abc123;"
	r := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	r.Header.Set("Cookie", cookie)
	w := httptest.NewRecorder()

	h1 := sm.Handler(h)
	h1.ServeHTTP(w, r)

	if status := w.Result().StatusCode; status != http.StatusInternalServerError {
		t.Fatalf("expected status '500' got '%d'", status)
	}
}

func TestErrorSaveSession(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)

	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, nil
	}

	store.set = func(string, []byte, time.Time) error {
		return errors.New("test")
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		sess.Set("hello", "world")
		w.Write([]byte("hello world"))
	})

	cookie := "session_id=abc123;"
	r := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	r.Header.Set("Cookie", cookie)
	w := httptest.NewRecorder()

	h1 := sm.Handler(h)
	h1.ServeHTTP(w, r)

	if cookie := w.Result().Header.Get("Set-Cookie"); cookie != "" {
		t.Fatal("expected no cookie but got one")
	}
}

func TestErrorDeleteSession(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)

	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, nil
	}

	store.set = func(string, []byte, time.Time) error {
		t.Fatal("unexpected call to store set")
		return nil
	}

	store.delete = func(string) error {
		return errors.New("test")
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		sess.Destroy()
		w.Write([]byte("hello world"))
	})

	cookie := "session_id=abc123;"
	r := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	r.Header.Set("Cookie", cookie)
	w := httptest.NewRecorder()

	h1 := sm.Handler(h)
	h1.ServeHTTP(w, r)

	if cookie := w.Result().Header.Get("Set-Cookie"); cookie != "" {
		t.Fatal("expected no cookie but got one")
	}
}

func TestDestroySession(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)

	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, nil
	}

	store.set = func(string, []byte, time.Time) error {
		t.Fatal("set called")
		return nil
	}

	var called bool
	store.delete = func(string) error {
		called = true
		return nil
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		sess.Destroy()
		w.Write([]byte("hello world"))
	})

	r := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	w := httptest.NewRecorder()

	h1 := sm.Handler(h)
	h1.ServeHTTP(w, r)

	if !called {
		t.Fatal("expected delete to be called")
	}
}

func TestSessionIdleTimeout(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)
	sm.SetIdleTimeout(10 * time.Minute)

	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, errors.New("test")
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	})

	r := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	w := httptest.NewRecorder()

	h1 := sm.Handler(h)
	h1.ServeHTTP(w, r)

	cookie := w.Result().Cookies()[0]

	expected := time.Now().Add(11 * time.Minute)
	if !cookie.Expires.IsZero() && cookie.Expires.After(expected) {
		t.Fatalf("exptected cookie expiration '%s' to be greater than '%s'", expected.UTC(), cookie.Expires.UTC())
	}
}

func TestSessionValues(t *testing.T) {
	store := &mockstore{}
	sm := httpx.NewSessionManager(store)

	store.get = func(string) ([]byte, bool, error) {
		return []byte{}, false, nil
	}

	store.set = func(token string, data []byte, _ time.Time) error {
		return nil
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sm.Get(r)
		if v := sess.Get("key"); v != nil {
			t.Fatalf("expected 'nil' got '%v'", v)
		}

		if v := sess.GetInt("int"); v != 0 {
			t.Fatalf("expected '0' got '%d'", v)
		}

		if v := sess.GetUint("uint"); v != 0 {
			t.Fatalf("expected '0' got '%d'", v)
		}

		if v := sess.GetFloat32("float32"); v != 0 {
			t.Fatalf("expected '0' got '%f'", v)
		}

		if v := sess.GetFloat64("float64"); v != 0 {
			t.Fatalf("expected '0' got '%f'", v)
		}

		if v := sess.GetString("string"); v != "" {
			t.Fatalf("expected '' got '%s'", v)
		}

		if v := sess.GetBool("bool"); v != false {
			t.Fatalf("expected 'false' got '%v'", v)
		}

		sess.Set("int", 1)
		sess.Set("uint", uint(2))
		sess.Set("float32", float32(3))
		sess.Set("float64", float64(4))
		sess.Set("string", "hello")
		sess.Set("bool", true)

		if v := sess.GetInt("int"); v != 1 {
			t.Fatalf("expected '1' got '%d'", v)
		}

		if v := sess.GetUint("uint"); v != 2 {
			t.Fatalf("expected '2' got '%d'", v)
		}

		if v := sess.GetFloat32("float32"); v != 3 {
			t.Fatalf("expected '3' got '%f'", v)
		}

		if v := sess.GetFloat64("float64"); v != 4 {
			t.Fatalf("expected '4' got '%f'", v)
		}

		if v := sess.GetString("string"); v != "hello" {
			t.Fatalf("expected 'hello' got '%s'", v)
		}

		if v := sess.GetBool("bool"); v != true {
			t.Fatalf("expected 'true' got '%v'", v)
		}

		sess.Delete("bool")
		if v := sess.GetBool("bool"); v != false {
			t.Fatalf("expected 'false' got '%v'", v)
		}
	})

	r := httptest.NewRequest("POST", "/", &bytes.Buffer{})
	w := httptest.NewRecorder()

	h1 := sm.Handler(h)
	h1.ServeHTTP(w, r)
}
