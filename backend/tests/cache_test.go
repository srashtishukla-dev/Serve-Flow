package tests_test

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/serveflow/serveflow/backend/internal/cache"
	"github.com/serveflow/serveflow/backend/internal/config"
	"github.com/serveflow/serveflow/backend/internal/routes"
	"github.com/serveflow/serveflow/backend/internal/services"
	"github.com/serveflow/serveflow/backend/internal/users"
)

func TestServiceCacheHitTenantIsolationAndInvalidation(t *testing.T) {
	server := newAuthTestServer(t)
	cacheClient := newIntegrationRedisClient(t)
	emailA := "service-cache-a+" + testEmail()
	emailB := "service-cache-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	userA, tokenA := createServiceTestUser(t, server, emailA)
	_, tokenB := createServiceTestUser(t, server, emailB)
	cachedRepository := services.NewRepository(server.pool, cacheClient)
	server.services = cachedRepository
	server.handler = routes.New(nil, server.tokens, users.NewRepository(server.pool), server.organizations,
		cachedRepository, server.customers, server.bookings, server.technicians, server.invoices,
		server.notifications, nil, server.analytics)

	first := requestService(t, server, http.MethodPost, "/api/v1/services", tokenA, cacheServicePayload("First service"))
	if first.Code != http.StatusCreated {
		t.Fatalf("create first service: %d %s", first.Code, first.Body.String())
	}
	firstService := decodeCreatedService(t, first)
	if list := requestService(t, server, http.MethodGet, "/api/v1/services", tokenA, nil); list.Code != http.StatusOK || len(decodeServiceList(t, list).Services) != 1 {
		t.Fatalf("cache miss service list failed: %d %s", list.Code, list.Body.String())
	}

	if _, err := server.pool.Exec(context.Background(), `
		INSERT INTO services (organization_id, name, duration_minutes, price)
		VALUES ($1::uuid, 'Direct database service', 30, 12.00)
	`, userA.OrganizationID); err != nil {
		t.Fatalf("insert uncached service fixture: %v", err)
	}
	cachedList := requestService(t, server, http.MethodGet, "/api/v1/services", tokenA, nil)
	if cachedList.Code != http.StatusOK || len(decodeServiceList(t, cachedList).Services) != 1 {
		t.Fatalf("cache hit did not return the stored service list: %d %s", cachedList.Code, cachedList.Body.String())
	}

	update := requestService(t, server, http.MethodPut, "/api/v1/services/"+firstService.ID, tokenA, cacheServicePayload("Updated service"))
	if update.Code != http.StatusOK {
		t.Fatalf("update service fixture: %d %s", update.Code, update.Body.String())
	}
	updatedList := requestService(t, server, http.MethodGet, "/api/v1/services", tokenA, nil)
	if updatedList.Code != http.StatusOK || len(decodeServiceList(t, updatedList).Services) != 2 {
		t.Fatalf("update did not invalidate service cache: %d %s", updatedList.Code, updatedList.Body.String())
	}
	if deleted := requestService(t, server, http.MethodDelete, "/api/v1/services/"+firstService.ID, tokenA, nil); deleted.Code != http.StatusNoContent {
		t.Fatalf("delete service fixture: %d %s", deleted.Code, deleted.Body.String())
	}
	deletedList := requestService(t, server, http.MethodGet, "/api/v1/services", tokenA, nil)
	if deletedList.Code != http.StatusOK || len(decodeServiceList(t, deletedList).Services) != 1 {
		t.Fatalf("delete did not invalidate service cache: %d %s", deletedList.Code, deletedList.Body.String())
	}
	second := requestService(t, server, http.MethodPost, "/api/v1/services", tokenA, cacheServicePayload("Second service"))
	if second.Code != http.StatusCreated {
		t.Fatalf("create second service: %d %s", second.Code, second.Body.String())
	}
	createdList := requestService(t, server, http.MethodGet, "/api/v1/services", tokenA, nil)
	if createdList.Code != http.StatusOK || len(decodeServiceList(t, createdList).Services) != 2 {
		t.Fatalf("create did not invalidate service cache: %d %s", createdList.Code, createdList.Body.String())
	}

	if list := requestService(t, server, http.MethodGet, "/api/v1/services", tokenB, nil); list.Code != http.StatusOK || len(decodeServiceList(t, list).Services) != 0 {
		t.Fatalf("organization B received organization A service cache: %d %s", list.Code, list.Body.String())
	}
	serviceB := requestService(t, server, http.MethodPost, "/api/v1/services", tokenB, cacheServicePayload("Organization B service"))
	if serviceB.Code != http.StatusCreated {
		t.Fatalf("create organization B service: %d %s", serviceB.Code, serviceB.Body.String())
	}
	listB := requestService(t, server, http.MethodGet, "/api/v1/services", tokenB, nil)
	listA := requestService(t, server, http.MethodGet, "/api/v1/services", tokenA, nil)
	if len(decodeServiceList(t, listB).Services) != 1 || len(decodeServiceList(t, listA).Services) != 2 {
		t.Fatalf("tenant cache lists crossed: A=%s B=%s", listA.Body.String(), listB.Body.String())
	}
}

func TestServiceListFallsBackWhenRedisIsUnavailable(t *testing.T) {
	server := newAuthTestServer(t)
	email := "service-cache-down+" + testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	_, token := createServiceTestUser(t, server, email)
	if response := requestService(t, server, http.MethodPost, "/api/v1/services", token, cacheServicePayload("PostgreSQL service")); response.Code != http.StatusCreated {
		t.Fatalf("create PostgreSQL service fixture: %d %s", response.Code, response.Body.String())
	}
	unavailableClient := cache.NewClient(config.Redis{Host: "127.0.0.1", Port: 1}, log.New(io.Discard, "", 0))
	t.Cleanup(func() { _ = unavailableClient.Close() })
	server.services = services.NewRepository(server.pool, unavailableClient)
	server.handler = routes.New(nil, server.tokens, users.NewRepository(server.pool), server.organizations,
		server.services, server.customers, server.bookings, server.technicians, server.invoices,
		server.notifications, nil, server.analytics)

	response := requestService(t, server, http.MethodGet, "/api/v1/services", token, nil)
	if response.Code != http.StatusOK || len(decodeServiceList(t, response).Services) != 1 {
		t.Fatalf("service list did not fall back to PostgreSQL: %d %s", response.Code, response.Body.String())
	}
}

func newIntegrationRedisClient(t *testing.T) *cache.Client {
	t.Helper()
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 6379
	if value := os.Getenv("REDIS_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 65535 {
			t.Fatalf("invalid REDIS_PORT for integration test: %q", value)
		}
		port = parsed
	}
	client := cache.NewClient(config.Redis{Host: host, Port: uint16(port)}, log.New(io.Discard, "", 0))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		_ = client.Close()
		t.Skipf("Redis integration test requires Redis at %s:%d: %v", host, port, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func cacheServicePayload(name string) map[string]any {
	return map[string]any{"name": name, "description": "", "duration_minutes": 30, "price": json.Number("10.00")}
}

func decodeServiceList(t *testing.T, response *httptest.ResponseRecorder) struct {
	Services []services.Service `json:"services"`
} {
	t.Helper()
	var body struct {
		Services []services.Service `json:"services"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode service list: %v", err)
	}
	return body
}
