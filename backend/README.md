# ServeFlow API

The Go API uses `net/http` and pgx. It reads connection settings from environment variables, verifies PostgreSQL connectivity before listening, and closes the pool after graceful HTTP shutdown.

## Configuration

Required: `POSTGRES_PASSWORD`.

Optional values default to `HTTP_PORT=8080`, `POSTGRES_HOST=localhost`, `POSTGRES_PORT=5433`, `POSTGRES_DB=serveflow`, `POSTGRES_USER=serveflow`, `CORS_ALLOWED_ORIGIN=http://localhost:5173`, `REDIS_HOST=localhost`, and `REDIS_PORT=6379`. Redis is an optional cache; the API continues using PostgreSQL if Redis is unavailable. See [`.env.example`](.env.example).

`GET /api/v1/services` uses a tenant-scoped cache-aside entry with a two-minute TTL. Creating, updating, or deleting a service invalidates that organization's service-list entry.

## Run

Start PostgreSQL and Redis from the repository root with `docker compose up -d`, then load the root `.env` values in PowerShell and run:

```powershell
Get-Content ..\.env | Where-Object { $_ -match '^\s*[^#].*=' } | ForEach-Object {
  $name, $value = $_ -split '=', 2
  Set-Item -Path "Env:$name" -Value $value
}
go run ./cmd/server
```

## Endpoint

- `GET /api/v1/health` — API health check

## Verify

```powershell
go vet ./...
go test ./...
go build ./...
```

PostgreSQL integration tests run only when `SERVEFLOW_TEST_DATABASE` names a dedicated database ending in `_test`. Set it to a disposable database such as `serveflow_test` and set `POSTGRES_PASSWORD` before running tests; without that explicit setting, database-backed tests are skipped.