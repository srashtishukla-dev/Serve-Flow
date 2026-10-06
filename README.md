# ServeFlow

ServeFlow is a multi-tenant service management application with a React frontend, Go REST API, PostgreSQL source of truth, optional Redis cache, and optional asynchronous SMTP delivery. Existing modules include authentication/workspaces, services, customers, bookings, technicians, invoices/payments, notifications, and analytics.

## Technology

- Frontend: React, Vite, React Router, Tailwind CSS, Lucide React, and Axios
- Backend: Go, `net/http`, and pgx
- Data and delivery: PostgreSQL, Redis cache, and optional SMTP email
- Local infrastructure: Docker Compose for PostgreSQL and Redis

## Architecture

The repository is a small monorepo. The React application is served independently by Vite and talks to the Go API through a shared Axios client. The Go server loads configuration from environment variables, opens and verifies a PostgreSQL connection pool at startup, treats Redis as an optional cache, and exposes versioned HTTP routes. Optional SMTP email is sent asynchronously through a PostgreSQL-backed delivery queue. Docker Compose manages PostgreSQL and Redis; the API and frontend run as local processes.

```text
serveflow/
├── backend/
│   ├── cmd/server/          # HTTP server entry point and lifecycle
│   ├── internal/config/     # Environment configuration
│   ├── internal/database/  # pgx connection pool
│   ├── internal/cache/     # Optional Redis cache
│   ├── internal/email/     # SMTP sender and durable delivery repository
│   ├── internal/jobs/      # Background notification/email worker
│   ├── internal/organizations/ # Organization model and repository
│   ├── internal/users/     # User model and repository
│   ├── internal/services/  # Tenant-aware service catalog model and repository
│   ├── internal/customers/ # Tenant-aware customer model and repository
│   ├── internal/bookings/  # Tenant-aware booking model and repository
│   ├── internal/technicians/ # Tenant-aware technician model and repository
│   ├── internal/invoices/  # Invoices and internal payment records
│   ├── internal/notifications/ # Tenant- and user-scoped notifications
│   ├── internal/analytics/ # Organization-scoped dashboard aggregates
│   ├── internal/handlers/  # HTTP endpoint handlers
│   ├── internal/handler/   # HTTP endpoint handlers
│   ├── internal/middleware/ # CORS, authentication, and request middleware
│   ├── internal/response/  # JSON response helpers
│   ├── migrations/         # SQL schema migrations
│   └── tests/              # API tests
├── frontend/
│   └── src/
│       ├── components/     # Shared UI and layout components
│       ├── pages/          # Landing and placeholder pages
│       ├── routes/         # React Router configuration
│       └── services/api/   # Shared Axios client
├── docker-compose.yml
└── .env.example
```

## Prerequisites

- Go 1.23 or later
- Node.js and npm
- Docker Desktop with Docker Compose (needed to run PostgreSQL)

## Configure and start PostgreSQL

From the repository root, create your local environment file and set a private development password:

```powershell
Copy-Item .env.example .env
```

Edit `.env`, then start PostgreSQL:

```powershell
docker compose up -d postgres
docker compose ps
```

The database data is stored in the `serveflow-postgres-data` named volume. No business tables are created on Day 1.

## Database migration

After PostgreSQL is running, apply migrations from the repository root in PowerShell. On a fresh database, run them in order:

```powershell
Get-Content .\backend\migrations\001_create_users.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\002_create_organizations_and_assign_users.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\003_create_services.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\004_create_customers.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\005_create_bookings.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\006_create_technicians.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\007_align_technician_schema.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\008_add_booking_technicians.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\009_create_invoices_and_payments.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\010_create_notifications.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
Get-Content .\backend\migrations\011_create_email_deliveries.sql | docker compose exec -T postgres psql -U serveflow -d serveflow
```

Migration 002 is additive and safe to rerun. It creates one legacy workspace per existing user, backfills `organization_id`, and then enforces the foreign key and `NOT NULL`; it does not drop or recreate tables. On an existing Day 3-5 database, apply only migration 002.

Migration 003 adds the tenant-owned services table with exact `NUMERIC(12,2)` prices and an organization-scoped index. On an existing Day 1-5 database, apply only migration 003.

Migration 004 adds customers with optional email, phone, and notes. Email is not globally unique; the same address may appear under different organizations. On an existing Day 1-6 database, apply only migration 004.

Migration 005 adds organization-scoped bookings with customer/service foreign keys, date/time checks, allowed statuses, and conflict indexes. On an existing Day 1-7 database, apply only migration 005.

Migration 005 adds organization-scoped bookings with customer/service foreign keys, statuses, date/time validation, and conflict indexes. On an existing Day 1-7 database, apply only migration 005.

Migration 006 creates organization-owned technicians. Migration 007 adds the UUID default, organization-delete behavior, and the organization-scoped list index without recreating tables or deleting technician data. On an existing Day 1-8 database that already has migration 006 applied, apply only migration 007.

Migration 008 extends the existing bookings table with an optional technician assignment, a same-organization foreign key, and an assignment schedule index. It does not create a second appointments table. On an existing Day 1-9 database, apply only migration 008.

Migration 009 adds organization-scoped invoices, line items, and internal payment records, including organization-matched customer/appointment/payment foreign keys. On an existing Day 1-10 database, apply only migration 009.

Migration 010 adds organization-scoped in-app notifications. Migration 011 adds the durable SMTP delivery outbox and its bounded retry state. Migration 012 adds `users.role`. Migration 013 links users to customers/technicians (`users.customer_id`/`technician_id`, unique, organization-scoped). Migration 014 adds the ASSIGNED, IN_PROGRESS and NO_SHOW appointment statuses. Apply these after migrations 001-011 on an existing database (`start-dev.ps1` applies all of them idempotently).

Role access: ADMIN manages everything in its organization; CUSTOMER and TECHNICIAN logins are created by an admin via `POST /api/v1/customers/{id}/login` and `POST /api/v1/technicians/{id}/login` and can only use `/api/v1/appointments` (scoped to their own records). Status changes use `POST /api/v1/appointments/{id}/status`.

## Run the backend

In a PowerShell terminal from the repository root, load the root environment values into the current terminal, then start the server:

```powershell
Set-Location .\backend
Get-Content ..\.env | Where-Object { $_ -match '^\s*[^#].*=' } | ForEach-Object {
  $name, $value = $_ -split '=', 2
  Set-Item -Path "Env:$name" -Value $value
}
go run ./cmd/server
```

The API listens on port `8080` by default. Startup fails clearly if required database configuration is missing or PostgreSQL cannot be reached. The server handles Ctrl+C and SIGTERM with graceful HTTP shutdown before closing the pgx pool.

## Run the frontend

In a separate terminal:

```powershell
Set-Location .\frontend
Copy-Item .env.example .env
npm install
npm run dev
```

Open the Vite URL shown in the terminal, normally `http://localhost:5173`. The frontend API base URL is configured by `VITE_API_BASE_URL` in `frontend/.env`.

## Health API

`GET http://localhost:8080/api/v1/health`

```json
{
  "status": "ok",
  "service": "serveflow-api",
  "dependencies": {
    "database": "available",
    "cache": "available"
  }
}
```

The endpoint returns HTTP 200 with `Content-Type: application/json`; cache availability is reported separately because Redis is optional.

## OpenAPI

The current versioned API description is in [`backend/openapi.yaml`](backend/openapi.yaml).

## Checks

From the repository root, run the isolated backend test runner. It uses a disposable PostgreSQL database, preserving the normal development database:

```powershell
.\test.ps1
```

This runs PostgreSQL-backed Go tests, `go vet`, and `go build`; the temporary database is removed after the run. For a formatting-only check from `backend/`, use `gofmt -d` on the Go source files (it reports differences without rewriting them).

From `frontend/`:

```powershell
npm run build
```

From the repository root, validate the OpenAPI document and Compose configuration with:

```powershell
npx --yes @redocly/cli lint --extends=minimal backend/openapi.yaml
docker compose config --quiet
```

## Day 2

- Created reusable React UI components
- Added Login and Register pages
- Added frontend validation
- Added authentication route structure
- Prepared backend authentication endpoints
- Verified backend tests/build
- Verified frontend build
- Verified Docker Compose

## Day 3

- Added PostgreSQL users table
- Added bcrypt password hashing
- Implemented real registration API
- Implemented real login API
- Connected React authentication forms to backend
- Added PostgreSQL-backed authentication tests
- Verified Docker/PostgreSQL
- Verified Go tests/build
- Verified React build

## Day 4

- Added JWT access-token authentication using an environment-provided signing secret
- Added JWT middleware and protected `GET /api/v1/me`
- Added React authentication state and session verification
- Added a protected dashboard and logout
- Added authentication tests for token issue, validation, expiry, and public login
- Verified Docker/PostgreSQL
- Verified Go tests/build
- Verified React build

JWT access tokens expire after 15 minutes by default. Set `JWT_SECRET` to a random value of at least 32 characters in the ignored root `.env`; optionally configure `JWT_ACCESS_TOKEN_TTL`. Refresh tokens, OAuth, RBAC, and multi-tenancy are not implemented.

## Day 5

- Added the PostgreSQL organizations table and connected users with a foreign key
- Backfilled existing users into individual legacy workspaces without deleting data
- Added workspace creation during registration in the same transaction as user creation
- Added the authenticated organization ID to JWT claims using the database association
- Added protected `GET /api/v1/organization`
- Added an authenticated-organization context helper for future tenant-scoped queries
- Updated the dashboard to fetch and display the organization and workspace slug
- Added PostgreSQL-backed organization and rollback tests
- Verified Docker/PostgreSQL
- Verified Go tests/build
- Verified React build

Future organization-owned queries should obtain the organization ID from `auth.OrganizationIDFromContext` and use it as a SQL parameter (for example, `WHERE organization_id = $1`). This is a tenant-isolation foundation, not complete multi-tenancy or advanced authorization.

## Day 6

- Added the PostgreSQL services table with tenant foreign key, validation constraints, timestamps, and organization index
- Added tenant-scoped create, list, get, update, and delete APIs
- Added exact decimal price handling without floating-point persistence
- Added service request validation and rejected client-supplied organization ownership
- Added real-PostgreSQL tests for CRUD, authentication, validation, and cross-organization isolation
- Added a protected Services page with list, create, edit, and delete flows
- Added a Dashboard link to Services
- Verified JWT authentication and organization scoping
- Verified PostgreSQL and Docker Compose
- Verified Go tests, vet, and build
- Verified React production build

Every service query is scoped using the authenticated organization's ID. RBAC and other business modules are not implemented.

## Day 7

- Added the PostgreSQL customers table with an organization foreign key and organization-scoped index
- Added tenant-aware customer create, list, get, update, and delete APIs
- Added validation for customer name, optional email, phone, and notes
- Added tenant-isolation tests, including cross-organization CRUD rejection and same-email support across organizations
- Added a protected Customers page with a reusable create/edit form and delete action
- Added Customers navigation to the dashboard
- Verified the existing Services functionality and tenant-isolation tests
- Verified Docker/PostgreSQL
- Verified Go tests, vet, and build
- Verified React production build

Customer records are organization-owned. Customer accounts/login and RBAC are not implemented.

## Day 8

- Added the PostgreSQL bookings table with organization, customer, and service foreign keys
- Added tenant-aware booking create, list, get, update, and cancel APIs
- Added date/time validation and organization ownership checks for customers and services
- Added overlap prevention for active bookings; adjacent slots are allowed and cancelled bookings do not block slots
- Added PostgreSQL integration tests for booking CRUD, tenant isolation, validation, and scheduling conflicts
- Added a protected Bookings page with customer/service selection, editing, status, and cancellation
- Added Bookings navigation to the dashboard
- Verified Days 1-7 regression routes and tenant isolation
- Verified Docker/PostgreSQL
- Verified Go tests, vet, and build
- Verified React production build

Booking integration tests should use a dedicated disposable PostgreSQL database. The test fixture applies migrations and removes test-created users and their workspaces.

## Day 9

- Added technician management with organization-scoped create, list, get, update, and deactivation APIs
- Added tenant isolation and validation tests using an explicitly selected test database
- Added a protected Technicians page with create, edit, and deactivate actions
- Added technician navigation and organization-scoped dashboard totals

## Day 10

- Extended the existing bookings table and API with optional technician assignments
- Added appointment endpoints and same-technician overlap detection while retaining the `/bookings` API
- Added a protected Appointments view that reuses the existing booking form and schedule records
- Added appointment navigation and a dashboard appointment total

## Day 11

- Added tenant-scoped invoices, server-calculated line totals, and organization-unique invoice numbers
- Added internal payment recording with exact decimal amounts, remaining-balance checks, and automatic payment status updates
- Added a protected invoice list, detail view, edit/cancel flow, and payment form
- Added an outstanding-invoice dashboard summary

## Development Roadmap

- The current application includes JWT authentication, organization-scoped services, customers, bookings and appointments, technicians, invoices and internal payment records, notifications with queued email delivery, organization-scoped analytics, and optional Redis caching.
- The frontend provides protected workflows for the implemented modules; the landing-page dashboard and testimonials are illustrative, not live business data.
- Customer accounts, role-based authorization, external payment processing, tickets, comments, and attachments are not implemented.

# Local development

With Docker Desktop running and a root `.env` containing `POSTGRES_PASSWORD` and a `JWT_SECRET` of at least 32 characters, start the complete local app from PowerShell:

```powershell
.\start-dev.ps1
```

The script loads the ignored `.env` into its child processes, starts the Compose PostgreSQL and Redis services, applies the SQL migrations to the configured ServeFlow database, builds and starts the Go API, and starts Vite. PostgreSQL uses host port `5433`, Redis `6379`, the API `8080`, and Vite `5173`. Process logs and IDs are kept under the ignored `.serveflow-dev` directory.

Stop the app and its Compose database/cache services with:

```powershell
.\stop-dev.ps1
```

## Backend automated tests

From the repository root, run:

```powershell
.\test.ps1
```

The runner loads the ignored root `.env`, starts the Compose PostgreSQL and Redis services if needed, creates a uniquely named disposable database inside the ServeFlow PostgreSQL instance, then runs `go test -count=1 ./...`, `go vet ./...`, and `go build ./...`. The integration-test database is dropped afterward, including when a check fails. Tests never target the configured `serveflow` development database; the runner requires the existing PostgreSQL host port `5433` and does not stop or alter other database containers.

## SMTP email delivery

1. Start the ServeFlow PostgreSQL and Redis services from the repository root:

   ```powershell
   docker compose up -d --wait postgres redis
   ```

2. Copy `.env.example` to the ignored root `.env`, then replace its SMTP example host and sender with values from your provider. Set `EMAIL_PROVIDER=smtp`, `EMAIL_FROM`, `EMAIL_FROM_NAME`, `SMTP_HOST`, and `SMTP_PORT`. Port `465` uses implicit TLS; other supported ports use required STARTTLS. If your provider requires authentication, set both `SMTP_USERNAME` and `SMTP_PASSWORD`. Configure `SERVEFLOW_EMAIL_SMOKE_TEST_TO` with an inbox you control. Keep credentials in `.env`, never in frontend configuration or source control.

3. Start the local application:

   ```powershell
   .\start-dev.ps1
   ```

4. In a PowerShell terminal, load the ignored `.env` values into that terminal and run the controlled smoke test:

   ```powershell
   Get-Content .env | ForEach-Object {
     $line = $_.Trim()
     if ($line -and -not $line.StartsWith('#') -and $line -match '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
       $name = $matches[1]
       $value = $matches[2].Trim()
       if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
         $value = $value.Substring(1, $value.Length - 2)
       }
       Set-Item -Path "Env:$name" -Value $value
     }
   }
   Set-Location .\backend
   go run ./cmd/email-smoke-test
   ```

5. Check the controlled recipient inbox, including spam/junk folders. SMTP acceptance means the relay accepted the message; it does not prove inbox delivery.

The smoke test requires `SERVEFLOW_EMAIL_SMOKE_TEST_TO`, validates it before connecting, and sends one fixed non-sensitive message through the same SMTP sender as the backend. It does not run during automated tests. `APP_ENV=production` requires SMTP mode and valid settings or the backend refuses to start.

Account registration queues a welcome email. Existing appointment-created, invoice-created, and payment-recorded events also create an in-app notification and a persisted email delivery record addressed to the organization's account user. Delivery is asynchronous and recorded as `queued`, `processing`, `sent`, or `failed`, with at most three attempts. A successful SMTP acceptance is recorded as `sent`; that does not prove inbox delivery or prevent downstream bounces.

## Validate the OpenAPI specification

From the repository root, run Redocly CLI's parser and structural checks:

```powershell
npx --yes @redocly/cli lint --extends=minimal backend/openapi.yaml
```

This uses a temporary CLI package and does not add a runtime dependency to the frontend or backend.
