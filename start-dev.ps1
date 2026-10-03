$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$envFile = Join-Path $repoRoot '.env'
$stateDirectory = Join-Path $repoRoot '.serveflow-dev'
$logDirectory = Join-Path $stateDirectory 'logs'
$stateFile = Join-Path $stateDirectory 'processes.json'
$backendDirectory = Join-Path $repoRoot 'backend'
$frontendDirectory = Join-Path $repoRoot 'frontend'
$backendExecutable = Join-Path $backendDirectory 'bin\serveflow-api.exe'
$viteScript = Join-Path $frontendDirectory 'node_modules\vite\bin\vite.js'

if (-not (Test-Path $envFile)) {
    throw 'Create a root .env with POSTGRES_PASSWORD and JWT_SECRET before starting ServeFlow.'
}

Get-Content $envFile | ForEach-Object {
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

if (-not $env:POSTGRES_PASSWORD) {
    throw 'POSTGRES_PASSWORD must be set in the root .env.'
}
if (-not $env:JWT_SECRET -or $env:JWT_SECRET.Length -lt 32) {
    throw 'JWT_SECRET must contain at least 32 characters in the root .env.'
}
if ($env:POSTGRES_PORT -and $env:POSTGRES_PORT -ne '5433') {
    throw 'POSTGRES_PORT must remain 5433 for the existing ServeFlow database.'
}

if (-not $env:POSTGRES_HOST) { $env:POSTGRES_HOST = 'localhost' }
if (-not $env:POSTGRES_PORT) { $env:POSTGRES_PORT = '5433' }
if (-not $env:POSTGRES_DB) { $env:POSTGRES_DB = 'serveflow' }
if (-not $env:POSTGRES_USER) { $env:POSTGRES_USER = 'serveflow' }
if (-not $env:REDIS_HOST) { $env:REDIS_HOST = 'localhost' }
if (-not $env:REDIS_PORT) { $env:REDIS_PORT = '6379' }
if (-not $env:HTTP_PORT) { $env:HTTP_PORT = '8080' }
if (-not $env:CORS_ALLOWED_ORIGIN) { $env:CORS_ALLOWED_ORIGIN = 'http://localhost:5173' }

if (Test-Path $stateFile) {
    $oldState = Get-Content $stateFile -Raw | ConvertFrom-Json
    $running = @($oldState.processes | Where-Object { Get-Process -Id $_.id -ErrorAction SilentlyContinue })
    if ($running.Count -gt 0) {
        throw 'ServeFlow development processes are already running. Use .\stop-dev.ps1 before starting them again.'
    }
    Remove-Item -LiteralPath $stateFile -Force
}

foreach ($port in @([int]$env:HTTP_PORT, 5173)) {
    $listener = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($listener) {
        throw "Port $port is already in use. Stop the existing ServeFlow process before starting another one."
    }
}

if (-not (Test-Path $viteScript)) {
    throw 'Frontend dependencies are missing. Run npm ci from the frontend directory first.'
}

New-Item -ItemType Directory -Path $logDirectory -Force | Out-Null
Push-Location $repoRoot
try {
    & docker compose up -d --wait postgres redis
    if ($LASTEXITCODE -ne 0) {
        throw 'Docker Compose could not start PostgreSQL and Redis.'
    }

    $migrationDirectory = Join-Path $backendDirectory 'migrations'
    foreach ($migration in Get-ChildItem -Path $migrationDirectory -Filter '*.sql' | Sort-Object Name) {
        if ($migration.Name -eq '006_create_technicians.sql') {
            $techniciansExist = & docker compose exec -T postgres psql -U $env:POSTGRES_USER -d $env:POSTGRES_DB -Atc "SELECT to_regclass('public.technicians') IS NOT NULL"
            if ($LASTEXITCODE -ne 0) {
                throw 'Could not check whether the technician migration has already been applied.'
            }
            if ($techniciansExist.Trim() -eq 't') { continue }
        }

        [System.IO.File]::ReadAllText($migration.FullName) | & docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U $env:POSTGRES_USER -d $env:POSTGRES_DB
        if ($LASTEXITCODE -ne 0) {
            throw "Database migration $($migration.Name) failed."
        }
    }
} finally {
    Pop-Location
}

Push-Location $backendDirectory
try {
    & go build -o $backendExecutable ./cmd/server
    if ($LASTEXITCODE -ne 0) {
        throw 'The Go backend build failed.'
    }
} finally {
    Pop-Location
}

$processes = @()
try {
    $backendProcess = Start-Process -FilePath $backendExecutable -WorkingDirectory $backendDirectory -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDirectory 'backend.log') -RedirectStandardError (Join-Path $logDirectory 'backend-error.log') -PassThru
    $processes += [pscustomobject]@{ name = 'backend'; id = $backendProcess.Id }

    $backendReady = $false
    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        if ($backendProcess.HasExited) { break }
        try {
            $health = Invoke-RestMethod -Uri "http://localhost:$($env:HTTP_PORT)/api/v1/health" -TimeoutSec 2
            if ($health.status -eq 'ok' -and $health.dependencies.database -eq 'available') {
                $backendReady = $true
                break
            }
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $backendReady) {
        throw "Backend did not become healthy. Check $logDirectory\backend-error.log."
    }

    $node = (Get-Command node.exe -ErrorAction Stop).Source
    $frontendProcess = Start-Process -FilePath $node -ArgumentList @($viteScript, '--host', '127.0.0.1') -WorkingDirectory $frontendDirectory -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDirectory 'frontend.log') -RedirectStandardError (Join-Path $logDirectory 'frontend-error.log') -PassThru
    $processes += [pscustomobject]@{ name = 'frontend'; id = $frontendProcess.Id }

    $frontendReady = $false
    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        if ($frontendProcess.HasExited) { break }
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:5173/' -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                $frontendReady = $true
                break
            }
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $frontendReady) {
        throw "Frontend did not become available. Check $logDirectory\frontend-error.log."
    }

    [pscustomobject]@{ processes = $processes } | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath $stateFile -Encoding UTF8
    Write-Output 'ServeFlow is running:'
    Write-Output "  API:      http://localhost:$($env:HTTP_PORT)"
    Write-Output '  Frontend: http://localhost:5173'
    Write-Output "  Logs:     $logDirectory"
} catch {
    foreach ($process in $processes) {
        Stop-Process -Id $process.id -ErrorAction SilentlyContinue
    }
    if (Test-Path $stateFile) { Remove-Item -LiteralPath $stateFile -Force }
    throw
}
