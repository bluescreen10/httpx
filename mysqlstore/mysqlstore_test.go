package mysqlstore_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"ithub.com/bluescreen10/httpx/mysqlstore"
)

func TestSetGet(t *testing.T) {
	token := "abc123"
	expectedData := []byte("hello world")

	db, err := getDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s, err := mysqlstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Set(token, expectedData, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
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

	db, err := getDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s, err := mysqlstore.New(db)
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

	db, err := getDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s, err := mysqlstore.New(db)
	if err != nil {
		t.Fatal(err)
	}

	err = s.Set(token, expectedData, time.Now().Add(1*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
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

	db, err := getDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s, err := mysqlstore.New(db)
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

func TestPeriodicCleanup(t *testing.T) {
	token1 := "abc123"
	token2 := "abc1234"
	expectedData := []byte("hello world")

	db, err := getDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s, err := mysqlstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	s.Set(token1, expectedData, time.Now().Add(1*time.Hour))
	s.Set(token2, expectedData, time.Now().Add(10*time.Millisecond))

	stop := make(chan (struct{}))
	go s.PeriodicCleanUp(20*time.Millisecond, stop)
	time.Sleep(50 * time.Millisecond)
	stop <- struct{}{}
	stmt := "SELECT COUNT(*) FROM sessions"
	row := db.QueryRow(stmt)

	var got int
	err = row.Scan(&got)
	if err != nil {
		t.Fatal(err)
	}

	if got != 1 {
		t.Fatalf("expected 1 item but got '%d'", got)
	}
}

func getDB(t *testing.T) (*sql.DB, error) {
	ctx := context.Background()
	server, err := testcontainers.Run(
		ctx, "mariadb:latest",
		testcontainers.WithEnv(map[string]string{
			"MARIADB_ROOT_PASSWORD": "rootpass",
			"MARIADB_DATABASE":      "testdb",
			"MARIADB_USER":          "testuser",
			"MARIADB_PASSWORD":      "testpass",
		}),
		testcontainers.WithExposedPorts("3306/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("3306/tcp"),
			wait.ForLog("ready for connections"),
		),
	)
	if err != nil {
		return nil, err
	}
	testcontainers.CleanupContainer(t, server)

	host, err := server.Host(ctx)
	if err != nil {
		return nil, err
	}
	port, err := server.MappedPort(ctx, "3306")
	if err != nil {
		return nil, err
	}

	// Build DSN
	dsn := fmt.Sprintf("testuser:testpass@tcp(%s:%s)/testdb?parseTime=true", host, port.Port())

	// Open DB connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}
