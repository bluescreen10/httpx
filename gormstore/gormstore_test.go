package gormstore_test

import (
	"testing"
	"time"

	"github.com/bluescreen10/httpx/gormstore"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetGet(t *testing.T) {
	token := "abc123"
	expectedData := []byte("hello world")

	db, err := getDB()
	if err != nil {
		t.Fatal(err)
	}

	s, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}

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

	db, err := getDB()
	if err != nil {
		t.Fatal(err)
	}

	s, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}

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

	db, err := getDB()
	if err != nil {
		t.Fatal(err)
	}

	s, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}

	s.Set(token, expectedData, time.Now().Add(-1*time.Hour))
	_, found, err := s.Get(token)

	if err != nil {
		t.Fatal(err)
	}

	if found {
		t.Fatalf("expected 'false' got '%v'", found)
	}
}

func TestDelete(t *testing.T) {
	token := "abc123"
	expectedData := []byte("hello world")

	db, err := getDB()
	if err != nil {
		t.Fatal(err)
	}

	s, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}

	s.Set(token, expectedData, time.Now().Add(1*time.Hour))
	if err := s.Delete(token); err != nil {
		t.Fatal(err)
	}

	_, found, err := s.Get(token)
	if err != nil {
		t.Fatal(err)
	}

	if found {
		t.Fatalf("expected 'false' got '%v'", found)
	}
}

func getDB() (*gorm.DB, error) {
	return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
}

func TestPeriodicCleanup(t *testing.T) {
	token1 := "abc123"
	token2 := "abc1234"
	expectedData := []byte("hello world")

	db, err := getDB()
	if err != nil {
		t.Fatal(err)
	}

	s, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}

	s.Set(token1, expectedData, time.Now().Add(1*time.Hour))
	s.Set(token2, expectedData, time.Now().Add(10*time.Millisecond))

	stop := make(chan (struct{}))
	go s.PeriodicCleanUp(20*time.Millisecond, stop)
	time.Sleep(50 * time.Millisecond)
	stop <- struct{}{}

	var result struct {
		Count int
	}

	db.Raw("SELECT count(*) as count FROM sessions").Scan(&result)

	if result.Count != 1 {
		t.Fatalf("expected 1 item but got '%d'", result.Count)
	}
}
