package cache

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/serveflow/serveflow/backend/internal/config"
)

func TestRedisClientPingGetSetDeleteAndTTL(t *testing.T) {
	settings := redisTestSettings(t)
	client := NewClient(settings, log.New(io.Discard, "", 0))
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Skipf("Redis integration test requires Redis at %s:%d: %v", settings.Host, settings.Port, err)
	}

	key := fmt.Sprintf("serveflow:test:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	if err := client.Set(ctx, key, []byte("cached-value"), time.Second); err != nil {
		t.Fatalf("set cache value: %v", err)
	}
	value, found, err := client.Get(ctx, key)
	if err != nil || !found || string(value) != "cached-value" {
		t.Fatalf("cache get = %q, found %t, error %v", value, found, err)
	}
	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("delete cache value: %v", err)
	}
	if _, found, err := client.Get(ctx, key); err != nil || found {
		t.Fatalf("deleted cache key found = %t, error = %v", found, err)
	}

	expiringKey := key + ":ttl"
	if err := client.Set(ctx, expiringKey, []byte("expires"), 80*time.Millisecond); err != nil {
		t.Fatalf("set expiring cache value: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, found, err := client.Get(ctx, expiringKey)
		if err != nil {
			t.Fatalf("get expiring cache value: %v", err)
		}
		if !found {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cache value did not expire")
}

func redisTestSettings(t *testing.T) config.Redis {
	t.Helper()
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 6379
	if value := os.Getenv("REDIS_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 65535 {
			t.Fatalf("invalid REDIS_PORT for test: %q", value)
		}
		port = parsed
	}
	return config.Redis{Host: host, Port: uint16(port)}
}
