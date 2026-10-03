$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$envFile = Join-Path $repoRoot '.env'
$testDatabase = $null
$testDatabaseCreated = $false

if (-not (Test-Path -LiteralPath $envFile)) {
    throw 'Create a root .env file before running the backend tests.'
}

Get-Content -LiteralPath $envFile | ForEach-Object {
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
    throw 'POSTGRES_PASSWORD must be set in the root .env file.'
}
if (-not $env:POSTGRES_PORT) { $env:POSTGRES_PORT = '5433' }
if ($env:POSTGRES_PORT -ne '5433') {
    throw 'The test runner requires the existing ServeFlow PostgreSQL host port 5433.'
}
if (-not $env:POSTGRES_USER) { $env:POSTGRES_USER = 'serveflow' }
if (-not $env:POSTGRES_HOST) { $env:POSTGRES_HOST = 'localhost' }
if (-not $env:REDIS_HOST) { $env:REDIS_HOST = 'localhost' }
if (-not $env:REDIS_PORT) { $env:REDIS_PORT = '6379' }

$testDatabase = "serveflow_test_$([Guid]::NewGuid().ToString('N'))_test"

Push-Location $repoRoot
try {
    & docker compose up -d --wait postgres redis
    if ($LASTEXITCODE -ne 0) {
        throw 'Docker Compose could not start PostgreSQL and Redis for the test run.'
    }

    $createDatabaseSql = "CREATE DATABASE `"$testDatabase`""
    & docker compose exec -T postgres psql -U $env:POSTGRES_USER -d postgres -v ON_ERROR_STOP=1 -c $createDatabaseSql
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not create the uniquely named disposable test database.'
    }
    $testDatabaseCreated = $true

    $env:POSTGRES_DB = $testDatabase
    $env:SERVEFLOW_TEST_DATABASE = $testDatabase

    Push-Location (Join-Path $repoRoot 'backend')
    try {
        Write-Output 'Running backend tests against a newly created disposable database.'
        $testOutput = @(& go test -json -count=1 ./...)
        $testExitCode = $LASTEXITCODE
        $testEvents = @(
            foreach ($line in $testOutput) {
                try {
                    $line | ConvertFrom-Json -ErrorAction Stop
                } catch {
                    throw 'Could not parse structured Go test output.'
                }
            }
        )
        $finishedTests = @($testEvents | Where-Object { $_.Test -and $_.Action -in @('pass', 'fail', 'skip') })
        $passedTests = @($finishedTests | Where-Object { $_.Action -eq 'pass' }).Count
        $failedTests = @($finishedTests | Where-Object { $_.Action -eq 'fail' }).Count
        $skippedTests = @($finishedTests | Where-Object { $_.Action -eq 'skip' }).Count
        Write-Output "Go test cases: $($passedTests + $failedTests + $skippedTests); passed: $passedTests; failed: $failedTests; skipped: $skippedTests."
        if ($testExitCode -ne 0) {
            $finishedTests | Where-Object { $_.Action -eq 'fail' } | ForEach-Object {
                Write-Output "Failed test: $($_.Package) $($_.Test)"
            }
            $testEvents | Where-Object { $_.Action -eq 'output' -and $_.Test -and ($_.Test -in @($finishedTests | Where-Object { $_.Action -eq 'fail' } | ForEach-Object { $_.Test })) } | ForEach-Object {
                Write-Output $_.Output
            }
            throw 'The backend test suite failed.'
        }

        & go vet ./...
        if ($LASTEXITCODE -ne 0) {
            throw 'go vet reported an issue.'
        }

        & go build ./...
        if ($LASTEXITCODE -ne 0) {
            throw 'The backend build failed.'
        }
    } finally {
        Pop-Location
    }
} finally {
    if ($testDatabaseCreated) {
        $dropDatabaseSql = 'DROP DATABASE IF EXISTS "' + $testDatabase + '" WITH (FORCE)'
        & docker compose exec -T postgres psql -U $env:POSTGRES_USER -d postgres -v ON_ERROR_STOP=1 -c $dropDatabaseSql
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Could not remove disposable test database $testDatabase."
        } else {
            Write-Output 'Removed the disposable test database.'
        }
    }
    Pop-Location
}
