package memstore_test

import (
	"testing"
	"time"

	"github.com/bluescreen10/httpx/memstore"
)

func TestSetGet(t *testing.T) {
	token := "abc123"
	expectedData := []byte("hello world")

	s := memstore.New()
	s.Set(token, expectedData, time.Now().Add(1*time.Hour))
	data, found, err := s.Get(token)

	if err != nil {
		t.Fatal(err)
	}

	if string(data) != string(expectedData) {
		t.Fatalf("expected '%s' got '%s'", expectedData, data)
	}

	if !found {
		t.Fatalf("expected 'true' got '%v'", found)
	}
}

func TestEmptyGet(t *testing.T) {
	token := "abc123"

	s := memstore.New()
	_, found, err := s.Get(token)

	if err != nil {
		t.Fatal(err)
	}

	if found {
		t.Fatalf("expected 'false' got '%v'", found)
	}
}

func TestGetExpired(t *testing.T) {
	token := "abc123"
	expectedData := []byte("hello world")

	s := memstore.New()
	s.Set(token, expectedData, time.Now().Add(-1*time.Hour))
	_, found, err := s.Get(token)

	if err != nil {
		t.Fatal(err)
	}

	if found {
		t.Fatalf("expected 'false' got '%v'", found)
	}
}

func TestPeriodicCleanup(t *testing.T) {
	token1 := "abc123"
	token2 := "abc1234"
	expectedData := []byte("hello world")

	s := memstore.New()
	s.Set(token1, expectedData, time.Now().Add(1*time.Hour))
	s.Set(token2, expectedData, time.Now().Add(10*time.Millisecond))

	stop := make(chan (struct{}))
	go s.PeriodicCleanUp(20*time.Millisecond, stop)
	time.Sleep(50 * time.Millisecond)
	stop <- struct{}{}
	if count := s.Count(); count != 1 {
		t.Fatalf("expected 1 item but got '%d'", count)
	}
}
