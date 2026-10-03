package tests_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/analytics"
	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/config"
	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/database"
	"github.com/serveflow/serveflow/backend/internal/invoices"
	"github.com/serveflow/serveflow/backend/internal/notifications"
	"github.com/serveflow/serveflow/backend/internal/organizations"
	"github.com/serveflow/serveflow/backend/internal/routes"
	"github.com/serveflow/serveflow/backend/internal/services"
	"github.com/serveflow/serveflow/backend/internal/technicians"
	"github.com/serveflow/serveflow/backend/internal/users"
	"golang.org/x/crypto/bcrypt"
)

const testPassword = "serveflow-test-password"
const testJWTSecret = "serveflow-test-jwt-secret-with-more-than-32-bytes"

type authTestServer struct {
	handler       http.Handler
	pool          *pgxpool.Pool
	tokens        *auth.TokenManager
	organizations *organizations.Repository
	services      *services.Repository
	customers     *customers.Repository
	bookings      *bookings.Repository
	technicians   *technicians.Repository
	invoices      *invoices.Repository
	notifications *notifications.Repository
	analytics     *analytics.Repository
}

func newAuthTestServer(t *testing.T) *authTestServer {
	t.Helper()
	t.Setenv("JWT_SECRET", testJWTSecret)
	testDatabase := strings.TrimSpace(os.Getenv("SERVEFLOW_TEST_DATABASE"))
	if !strings.HasSuffix(strings.ToLower(testDatabase), "_test") {
		t.Skip("PostgreSQL integration tests require SERVEFLOW_TEST_DATABASE ending in _test")
	}
	t.Setenv("POSTGRES_DB", testDatabase)
	cfg, err := config.Load()
	if err != nil {
		if os.Getenv("POSTGRES_PASSWORD") == "" {
			t.Skip("PostgreSQL integration tests require POSTGRES_PASSWORD")
		}
		t.Fatalf("load test configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.Database)
	if err != nil {
		t.Fatalf("connect to test PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	for _, migrationName := range []string{"001_create_users.sql", "002_create_organizations_and_assign_users.sql", "003_create_services.sql", "004_create_customers.sql", "005_create_bookings.sql"} {
		migration, err := os.ReadFile("../migrations/" + migrationName)
		if err != nil {
			t.Fatalf("read migration %s: %v", migrationName, err)
		}
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migrationName, err)
		}
	}
	var techniciansTableExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'technicians'
	)`).Scan(&techniciansTableExists); err != nil {
		t.Fatalf("check for technicians table: %v", err)
	}
	if !techniciansTableExists {
		migration, err := os.ReadFile("../migrations/006_create_technicians.sql")
		if err != nil {
			t.Fatalf("read technician migration: %v", err)
		}
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply technician migration: %v", err)
		}
	}
	migration, err := os.ReadFile("../migrations/007_align_technician_schema.sql")
	if err != nil {
		t.Fatalf("read technician schema migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply technician schema migration: %v", err)
	}
	migration, err = os.ReadFile("../migrations/008_add_booking_technicians.sql")
	if err != nil {
		t.Fatalf("read booking technician migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply booking technician migration: %v", err)
	}
	migration, err = os.ReadFile("../migrations/009_create_invoices_and_payments.sql")
	if err != nil {
		t.Fatalf("read invoice migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply invoice migration: %v", err)
	}
	migration, err = os.ReadFile("../migrations/010_create_notifications.sql")
	if err != nil {
		t.Fatalf("read notification migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply notification migration: %v", err)
	}
	migration, err = os.ReadFile("../migrations/011_create_email_deliveries.sql")
	if err != nil {
		t.Fatalf("read email delivery migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply email delivery migration: %v", err)
	}

	repository := users.NewRepository(pool)
	organizationRepository := organizations.NewRepository(pool)
	serviceRepository := services.NewRepository(pool)
	customerRepository := customers.NewRepository(pool)
	bookingRepository := bookings.NewRepository(pool)
	technicianRepository := technicians.NewRepository(pool)
	invoiceRepository := invoices.NewRepository(pool)
	notificationRepository := notifications.NewRepository(pool)
	analyticsRepository := analytics.NewRepository(pool)
	tokenManager, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessTokenTTL)
	if err != nil {
		t.Fatalf("configure test JWT: %v", err)
	}
	service := auth.NewService(pool, repository, organizationRepository, tokenManager)
	return &authTestServer{
		handler:       routes.New(service, tokenManager, repository, organizationRepository, serviceRepository, customerRepository, bookingRepository, technicianRepository, invoiceRepository, notificationRepository, nil, analyticsRepository),
		pool:          pool,
		tokens:        tokenManager,
		organizations: organizationRepository,
		services:      serviceRepository,
		customers:     customerRepository,
		bookings:      bookingRepository,
		technicians:   technicianRepository,
		invoices:      invoiceRepository,
		notifications: notificationRepository,
		analytics:     analyticsRepository,
	}
}

func TestRegistrationSucceedsAndStoresOnlyHash(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })

	recorder := postJSON(t, server.handler, "/api/v1/auth/register", map[string]string{
		"name": "Test User", "email": email, "password": testPassword,
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	assertSafeUserResponse(t, recorder)

	var passwordHash string
	if err := queryPasswordHash(server, email, &passwordHash); err != nil {
		t.Fatalf("read stored password hash: %v", err)
	}
	if passwordHash == testPassword || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(testPassword)) != nil {
		t.Fatal("stored password is not a valid password hash")
	}
}

func TestRegistrationRejectsDuplicateEmail(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	registerTestUser(t, server, email)

	recorder := postJSON(t, server.handler, "/api/v1/auth/register", map[string]string{
		"name": "Test User", "email": email, "password": testPassword,
	})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestRegistrationRejectsInvalidRequest(t *testing.T) {
	server := newAuthTestServer(t)
	recorder := postJSON(t, server.handler, "/api/v1/auth/register", map[string]string{
		"name": "Test User", "email": "not-an-email", "password": testPassword,
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLoginSucceeds(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	registerTestUser(t, server, email)
	registeredUser, _, err := users.NewRepository(server.pool).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("find registered user: %v", err)
	}

	recorder := postJSON(t, server.handler, "/api/v1/auth/login", map[string]string{
		"email": email, "password": testPassword,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	assertSafeUserResponse(t, recorder)
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatal("successful login did not return a JWT")
	}
	principal, err := server.tokens.Verify(body.Token)
	if err != nil {
		t.Fatal("login returned an invalid JWT")
	}
	if principal.UserID != registeredUser.ID || principal.OrganizationID != registeredUser.OrganizationID {
		t.Fatal("login JWT claims do not match the user's server-side records")
	}
}

func TestOrganizationCanBeCreated(t *testing.T) {
	server := newAuthTestServer(t)
	ctx := context.Background()
	tx, err := server.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin organization transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	slug := fmt.Sprintf("test-organization-%d", time.Now().UnixNano())
	organization, err := server.organizations.Create(ctx, tx, "Test Organization", slug)
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit organization: %v", err)
	}
	t.Cleanup(func() {
		if _, err := server.pool.Exec(ctx, "DELETE FROM organizations WHERE id = $1", organization.ID); err != nil {
			t.Errorf("clean up test organization: %v", err)
		}
	})
	if organization.ID == "" || organization.Slug != slug {
		t.Fatalf("organization = %+v, expected an ID and slug %q", organization, slug)
	}
}

func TestRegistrationCreatesAndAssociatesWorkspace(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	registerTestUser(t, server, email)

	user, _, err := users.NewRepository(server.pool).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("find registered user: %v", err)
	}
	if user.OrganizationID == "" {
		t.Fatal("registered user has no organization")
	}
	organization, err := server.organizations.FindByID(context.Background(), user.OrganizationID)
	if err != nil {
		t.Fatalf("find registered user's organization: %v", err)
	}
	if organization.Name != "Test User's Workspace" || organization.Slug != "test-user-workspace" {
		t.Fatalf("organization = %+v, want Test User's Workspace / test-user-workspace", organization)
	}
}

func TestDuplicateRegistrationRollsBackOrganization(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	registerTestUser(t, server, email)

	var before int
	if err := server.pool.QueryRow(context.Background(), "SELECT count(*) FROM organizations").Scan(&before); err != nil {
		t.Fatalf("count organizations before duplicate: %v", err)
	}
	recorder := postJSON(t, server.handler, "/api/v1/auth/register", map[string]string{
		"name": "Another User", "email": email, "password": testPassword,
	})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	var after int
	if err := server.pool.QueryRow(context.Background(), "SELECT count(*) FROM organizations").Scan(&after); err != nil {
		t.Fatalf("count organizations after duplicate: %v", err)
	}
	if after != before {
		t.Fatalf("organization count = %d after failed registration, want %d", after, before)
	}
}

func TestValidJWTAccessesCurrentUser(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	registerTestUser(t, server, email)
	user, _, err := users.NewRepository(server.pool).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("find registered user: %v", err)
	}
	token, err := server.tokens.Issue(user.ID, user.OrganizationID)
	if err != nil {
		t.Fatalf("issue test JWT: %v", err)
	}
	recorder := getMe(t, server.handler, "Bearer "+token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /me response: %v", err)
	}
	if body["id"] == nil || body["name"] == nil || body["email"] == nil {
		t.Fatal("/me response is missing safe user fields")
	}
	if _, exists := body["password_hash"]; exists {
		t.Fatal("/me response contains a password hash")
	}
}

func TestOrganizationEndpointUsesAuthenticatedOrganization(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	registerTestUser(t, server, email)
	user, _, err := users.NewRepository(server.pool).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("find registered user: %v", err)
	}
	token, err := server.tokens.Issue(user.ID, user.OrganizationID)
	if err != nil {
		t.Fatalf("issue test JWT: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/organization?organization_id=00000000-0000-0000-0000-000000000000", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var organization organizations.Organization
	if err := json.Unmarshal(recorder.Body.Bytes(), &organization); err != nil {
		t.Fatalf("decode organization response: %v", err)
	}
	if organization.ID != user.OrganizationID || organization.Slug != "test-user-workspace" {
		t.Fatalf("organization = %+v, does not match authenticated user's organization", organization)
	}
}

func TestOrganizationEndpointRejectsUnauthenticatedRequest(t *testing.T) {
	server := newAuthTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/organization", nil)
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestCurrentUserRejectsMissingJWT(t *testing.T) {
	server := newAuthTestServer(t)
	recorder := getMe(t, server.handler, "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestCurrentUserRejectsInvalidJWT(t *testing.T) {
	server := newAuthTestServer(t)
	claims := jwt.RegisteredClaims{
		Subject:   "unknown-user",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	invalidToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret + "wrong-key"))
	if err != nil {
		t.Fatalf("sign invalid JWT: %v", err)
	}
	recorder := getMe(t, server.handler, "Bearer "+invalidToken)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestCurrentUserRejectsIncorrectAuthorizationFormat(t *testing.T) {
	server := newAuthTestServer(t)
	recorder := getMe(t, server.handler, "Token not-a-bearer-token")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestCurrentUserRejectsExpiredJWT(t *testing.T) {
	server := newAuthTestServer(t)
	claims := jwt.RegisteredClaims{
		Subject:   "expired-user",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("sign expired JWT: %v", err)
	}
	recorder := getMe(t, server.handler, "Bearer "+expiredToken)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoginRemainsPublic(t *testing.T) {
	server := newAuthTestServer(t)
	recorder := postJSON(t, server.handler, "/api/v1/auth/login", map[string]string{
		"email": "missing@example.test", "password": testPassword,
	})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want invalid-credentials response", recorder.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Error.Code != "INVALID_CREDENTIALS" {
		t.Fatal("login was intercepted by authentication middleware")
	}
}

func TestLoginRejectsIncorrectPassword(t *testing.T) {
	server := newAuthTestServer(t)
	email := testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	registerTestUser(t, server, email)

	recorder := postJSON(t, server.handler, "/api/v1/auth/login", map[string]string{
		"email": email, "password": "incorrect-password",
	})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoginRejectsUnknownEmail(t *testing.T) {
	server := newAuthTestServer(t)
	recorder := postJSON(t, server.handler, "/api/v1/auth/login", map[string]string{
		"email": testEmail(), "password": testPassword,
	})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func postJSON(t *testing.T, handler http.Handler, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func registerTestUser(t *testing.T, server *authTestServer, email string) {
	t.Helper()
	recorder := postJSON(t, server.handler, "/api/v1/auth/register", map[string]string{
		"name": "Test User", "email": email, "password": testPassword,
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("test user registration status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func assertSafeUserResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	user, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatal("response does not contain a user object")
	}
	if _, exists := user["password"]; exists {
		t.Fatal("response contains a password")
	}
	if _, exists := user["password_hash"]; exists {
		t.Fatal("response contains a password hash")
	}
	if user["id"] == nil || user["name"] == nil || user["email"] == nil {
		t.Fatal("response is missing safe user fields")
	}
}

func queryPasswordHash(server *authTestServer, email string, destination *string) error {
	return server.pool.QueryRow(context.Background(), "SELECT password_hash FROM users WHERE email = $1", email).Scan(destination)
}

func getMe(t *testing.T, handler http.Handler, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func cleanupUser(t *testing.T, server *authTestServer, email string) {
	t.Helper()
	ctx := context.Background()
	var organizationID string
	if err := server.pool.QueryRow(ctx, "SELECT organization_id::text FROM users WHERE email = $1", email).Scan(&organizationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		t.Errorf("find auth test organization: %v", err)
		return
	}
	if _, err := server.pool.Exec(ctx, "DELETE FROM users WHERE email = $1", email); err != nil {
		t.Errorf("clean up auth test user: %v", err)
		return
	}
	if _, err := server.pool.Exec(ctx, `
		DELETE FROM organizations
		WHERE id = $1
		  AND NOT EXISTS (SELECT 1 FROM users WHERE organization_id = $1)
	`, organizationID); err != nil {
		t.Errorf("clean up auth test organization: %v", err)
	}
}

func testEmail() string {
	return fmt.Sprintf("day3-%d@example.test", time.Now().UnixNano())
}
