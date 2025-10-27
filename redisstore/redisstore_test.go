package redisstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/bluescreen10/httpx/redisstore"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSetGet(t *testing.T) {
	token := "abc123"
	expectedData := []byte("hello world")

	rdb, err := getRedisDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s := redisstore.New(rdb)
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

	rdb, err := getRedisDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s := redisstore.New(rdb)
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

	rdb, err := getRedisDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s := redisstore.New(rdb)
	s.Set(token, expectedData, time.Now().Add(1*time.Millisecond))

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

	rdb, err := getRedisDB(t)
	if err != nil {
		t.Fatal(err)
	}

	s := redisstore.New(rdb)
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

func getRedisDB(t *testing.T) (*redis.Client, error) {
	ctx := context.Background()
	server, err := testcontainers.Run(
		ctx, "redis:latest",
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp"),
			wait.ForLog("Ready to accept connections"),
		),
	)
	if err != nil {
		return nil, err
	}
	testcontainers.CleanupContainer(t, server)
	endpoint, err := server.Endpoint(ctx, "")
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(&redis.Options{
		Addr: endpoint,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})
	return client, nil
}
