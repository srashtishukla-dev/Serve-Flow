# 🚀 ServeFlow

### Full-Stack Service Management Platform

ServeFlow is a **production-style, multi-tenant service management platform** built with **Go, React, PostgreSQL, and Redis**.

The platform is designed to help service-based organizations manage their complete operational workflow — including **customers, services, bookings, appointments, technicians, invoices, payments, notifications, and analytics** — through a secure and structured full-stack application.

> 🚧 **Project Status:** Actively developed and continuously improved.

---

## ✨ What ServeFlow Does

ServeFlow brings common service-management workflows into a single platform.

### Core Capabilities

* 🔐 JWT-based authentication
* 🏢 Organization-based multi-tenancy
* 👥 Customer management
* 🛠️ Service management
* 📅 Bookings and appointments
* 👨‍🔧 Technician management and assignment
* 💰 Invoices and internal payment records
* 🔔 In-app notifications
* 📧 Asynchronous SMTP email delivery
* 📊 Organization-scoped analytics
* ⚡ Optional Redis caching
* 🧪 Backend and frontend automated tests
* 📖 OpenAPI API documentation
* 🐳 Docker-based local infrastructure
* 🔄 GitHub Actions CI verification

---

# 📸 Application Showcase

The following screenshots show the current ServeFlow application interface and implemented workflows.

## 🏠 Home Page

![ServeFlow Home Page](docs/screenshots/home.png)

---

## ✨ Home Page Views

| Home View 1                                | Home View 2                                |
| ------------------------------------------ | ------------------------------------------ |
| ![Home View 1](docs/screenshots/home1.png) | ![Home View 2](docs/screenshots/home2.png) |

![Home View 3](docs/screenshots/home3.png)

---

## 🔐 Authentication

| Login                                | Registration                                       |
| ------------------------------------ | -------------------------------------------------- |
| ![Login](docs/screenshots/login.png) | ![Registration](docs/screenshots/registration.png) |

---

## 📊 Dashboard

![ServeFlow Dashboard](docs/screenshots/dashboard.png)

---

## 📅 Booking Management

![Booking Management](docs/screenshots/booking.png)

---

## 💰 Invoice Management

![Invoice Management](docs/screenshots/invoice.png)

---

## 📈 Analytics

![ServeFlow Analytics](docs/screenshots/analytics.png)

---

# 🏗️ Architecture

```text
                    ┌──────────────────────┐
                    │    React Frontend    │
                    │    Vite + Router     │
                    └──────────┬───────────┘
                               │
                               │ REST / JSON
                               ▼
                    ┌──────────────────────┐
                    │      Go REST API     │
                    │    net/http + pgx    │
                    └───────┬───────┬──────┘
                            │       │
                 ┌──────────┘       └──────────┐
                 ▼                             ▼
        ┌─────────────────┐           ┌─────────────────┐
        │   PostgreSQL    │           │      Redis      │
        │ Source of Truth │           │  Optional Cache │
        └─────────────────┘           └─────────────────┘
                 │
                 ▼
        ┌─────────────────┐
        │ Background Jobs │
        │ Email / Alerts  │
        └─────────────────┘
```

The application follows a monorepo structure with a React frontend and Go backend.

PostgreSQL is the primary source of truth. Redis is used as an optional caching layer, while asynchronous background jobs handle notification and email-delivery workflows.

---

# 🛠️ Technology Stack

## Frontend

* React
* JavaScript
* Vite
* React Router
* Tailwind CSS
* Lucide React
* Axios

## Backend

* Go
* `net/http`
* `pgx`
* REST APIs
* JWT authentication
* Middleware-based request handling

## Database & Infrastructure

* PostgreSQL
* Redis
* Docker
* Docker Compose

## Testing & Quality

* Go tests
* PostgreSQL integration tests
* React tests
* `go vet`
* Go build verification
* Frontend production build
* OpenAPI validation

## Development & CI

* Git
* GitHub
* GitHub Actions
* PowerShell development scripts

---

# 🔐 Authentication & Security

ServeFlow uses JWT-based authentication with protected API routes.

The backend includes:

* Password hashing
* JWT access tokens
* Authentication middleware
* Protected endpoints
* Role-based access foundations
* Organization-aware authorization
* Request validation
* Tenant isolation
* Cross-organization access tests

JWT configuration is provided through environment variables and secrets are kept outside source control.

---

# 🏢 Multi-Tenant Architecture

ServeFlow is designed around **organization-level data isolation**.

Business records are associated with an organization, and authenticated requests obtain the organization context from the authenticated user rather than trusting organization identifiers supplied by the client.

Tenant-aware modules include:

* Services
* Customers
* Bookings
* Appointments
* Technicians
* Invoices
* Payments
* Notifications
* Analytics

Cross-organization access is covered by backend integration tests.

---

# 👥 Customer Management

The customer module provides organization-scoped operations for:

* Creating customers
* Listing customers
* Viewing customer details
* Updating customer information
* Deleting customers
* Validating customer data

Customer records support information such as:

* Name
* Email
* Phone
* Notes

---

# 🛠️ Service Management

Organizations can manage their service catalog through protected APIs and frontend workflows.

Supported operations include:

* Create service
* List services
* View service
* Update service
* Delete service
* Validate service data
* Maintain organization ownership

Prices use exact decimal database handling rather than floating-point persistence.

---

# 📅 Bookings & Appointments

ServeFlow provides scheduling functionality built around organization-scoped bookings.

Features include:

* Booking creation
* Booking listing
* Booking updates
* Booking cancellation
* Customer/service selection
* Appointment scheduling
* Technician assignment
* Appointment status management
* Same-technician scheduling conflict detection
* Overlap prevention for active bookings
* Validation of scheduling data

Supported appointment workflow includes statuses such as:

```text
CREATED
ASSIGNED
IN_PROGRESS
COMPLETED
CANCELLED
NO_SHOW
```

The appointment system extends the existing booking model rather than maintaining a separate duplicate scheduling system.

---

# 👨‍🔧 Technician Management

Technicians are organization-owned resources.

The platform supports:

* Technician creation
* Technician listing
* Technician details
* Technician updates
* Technician deactivation
* Technician assignment to appointments
* Organization-level isolation

Technician-related workflows are covered by backend tests.

---

# 💰 Invoices & Payments

ServeFlow includes organization-scoped invoicing and internal payment tracking.

## Invoices

* Invoice creation
* Invoice listing
* Invoice details
* Invoice updates
* Invoice cancellation
* Server-calculated line totals
* Organization-unique invoice numbers
* Outstanding invoice tracking

## Payments

The current implementation supports **internal/manual payment recording**.

It includes:

* Exact decimal payment amounts
* Remaining-balance validation
* Automatic payment-status updates
* Payment records associated with invoices

> **Note:** External payment-gateway integration is not currently implemented.

---

# 🔔 Notifications & Email

ServeFlow includes an asynchronous notification workflow.

The system supports:

* In-app notifications
* Persistent email-delivery records
* Background notification jobs
* SMTP email delivery
* Delivery status tracking
* Retry handling

Email delivery states include:

```text
queued
processing
sent
failed
```

SMTP credentials remain in environment configuration and are not committed to the repository.

---

# 📊 Analytics

The backend includes organization-scoped analytics and dashboard aggregates.

Analytics are designed to provide operational visibility across the service-management workflow.

The current frontend includes an analytics page for the implemented analytics functionality.

---

# ⚡ Redis

Redis is integrated as an **optional caching layer**.

The application can report cache availability independently from PostgreSQL.

Redis is managed locally through Docker Compose.

---

# 🧪 Testing & Quality

ServeFlow includes automated backend and frontend tests.

Backend coverage includes areas such as:

* Authentication
* Roles
* Organizations
* Customers
* Services
* Bookings
* Appointments
* Technicians
* Invoices
* Notifications
* Analytics
* Redis/cache
* API quality
* Email delivery

The repository also includes:

* `go test`
* `go vet`
* `go build`
* Frontend production build
* OpenAPI validation
* Docker Compose configuration validation

The root `test.ps1` script creates a disposable PostgreSQL test database so integration tests do not use the normal development database.

---

# 📖 API Documentation

The backend API is documented using OpenAPI:

```text
backend/openapi.yaml
```

The API follows versioned routes such as:

```text
/api/v1/...
```

---

# 🐳 Docker & Local Infrastructure

Docker Compose is used for local infrastructure.

Current services include:

```text
PostgreSQL
Redis
```

The project also includes PowerShell scripts for starting and stopping the local development environment.

### Typical Local Ports

| Component  | Port |
| ---------- | ---: |
| PostgreSQL | 5433 |
| Redis      | 6379 |
| Go API     | 8080 |
| React/Vite | 5173 |

---

# 🔄 CI/CD

The repository includes a GitHub Actions verification workflow:

```text
.github/workflows/verify.yml
```

The workflow is used to automatically verify project changes.

The repository also contains development and verification scripts:

```text
start-dev.ps1
stop-dev.ps1
test.ps1
```

> CI workflow configuration is implemented in the repository. Individual GitHub Actions runs should be checked in GitHub to confirm their current pass/fail status.

---

# 📁 Project Structure

```text
ServeFlow/
│
├── backend/
│   ├── cmd/
│   │   ├── server/
│   │   └── email-smoke-test/
│   │
│   ├── internal/
│   │   ├── analytics/
│   │   ├── auth/
│   │   ├── bookings/
│   │   ├── cache/
│   │   ├── config/
│   │   ├── customers/
│   │   ├── database/
│   │   ├── email/
│   │   ├── handlers/
│   │   ├── invoices/
│   │   ├── jobs/
│   │   ├── middleware/
│   │   ├── notifications/
│   │   ├── organizations/
│   │   ├── pagination/
│   │   ├── response/
│   │   ├── routes/
│   │   ├── services/
│   │   ├── technicians/
│   │   └── users/
│   │
│   ├── migrations/
│   ├── tests/
│   ├── openapi.yaml
│   ├── go.mod
│   └── go.sum
│
├── frontend/
│   └── src/
│       ├── components/
│       ├── pages/
│       ├── routes/
│       ├── services/
│       ├── utils/
│       └── __tests__/
│
├── docs/
│   └── screenshots/
│       ├── home.png
│       ├── home1.png
│       ├── home2.png
│       ├── home3.png
│       ├── login.png
│       ├── registration.png
│       ├── dashboard.png
│       ├── booking.png
│       ├── invoice.png
│       └── analytics.png
│
├── .github/
│   └── workflows/
│
├── docker-compose.yml
├── start-dev.ps1
├── stop-dev.ps1
├── test.ps1
├── .env.example
└── README.md
```

---

# 🚀 Local Development

## Prerequisites

Install:

* Go 1.23+
* Node.js
* npm
* Docker Desktop
* Git

## 1. Configure Environment

Create a local environment file:

```powershell
Copy-Item .env.example .env
```

Edit `.env` and provide your local development values.

**Never commit the real `.env` file.**

## 2. Start the Application

From the repository root:

```powershell
.\start-dev.ps1
```

This starts the local infrastructure, applies database migrations, starts the Go API, and starts the React development server.

## 3. Stop the Application

```powershell
.\stop-dev.ps1
```

## 4. Run Tests

```powershell
.\test.ps1
```

## 5. Build the Frontend

```powershell
cd frontend
npm install
npm run build
```

---

# ❤️ Why I Built ServeFlow

I wanted to build something beyond a basic CRUD application.

Through ServeFlow, I am gaining practical experience with:

* Full-stack application architecture
* REST API development
* Go backend engineering
* React frontend development
* PostgreSQL data modeling
* Redis caching
* JWT authentication
* Multi-tenant application design
* Role-based access
* Background jobs
* Email delivery
* Automated testing
* Docker
* GitHub Actions
* OpenAPI
* Git-based development workflows

The project is being developed incrementally with an emphasis on **building, testing, verifying, and improving each feature**.

---

# 🗺️ Current Development Status

## Implemented

* React frontend
* Go REST API
* PostgreSQL
* Redis integration
* JWT authentication
* Organization-based data isolation
* Services
* Customers
* Bookings
* Appointments
* Technicians
* Invoices
* Internal payment records
* Notifications
* SMTP email delivery
* Analytics
* Automated tests
* Docker infrastructure
* OpenAPI documentation
* GitHub Actions workflow

## Future Improvements

* External payment gateway integration
* Additional customer and technician workflows
* Further production hardening
* Additional UI/UX refinement
* Expanded reporting and analytics

---

# 👩‍💻 Developer

**Srashti Shukla**

MCA Graduate | Software Engineer

Interested in:

**Java • Go • Python • Full-Stack Development • REST APIs • Backend Engineering**

---

⭐ **ServeFlow is an actively developed full-stack project focused on solving realistic service-management problems while applying production-oriented software engineering practices.**

