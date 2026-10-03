$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$stateFile = Join-Path $repoRoot '.serveflow-dev\processes.json'
$backendExecutable = [System.IO.Path]::GetFullPath((Join-Path $repoRoot 'backend\bin\serveflow-api.exe'))
$viteScript = [System.IO.Path]::GetFullPath((Join-Path $repoRoot 'frontend\node_modules\vite\bin\vite.js'))
$nodePath = (Get-Command node.exe -ErrorAction SilentlyContinue).Source

if (Test-Path $stateFile) {
    $state = Get-Content $stateFile -Raw | ConvertFrom-Json
    foreach ($process in $state.processes) {
        $runningProcess = Get-CimInstance Win32_Process -Filter "ProcessId = $($process.id)" -ErrorAction SilentlyContinue
        if (-not $runningProcess) { continue }

        $isServeFlowProcess = $false
        if ($process.name -eq 'backend' -and $runningProcess.ExecutablePath -and [System.IO.Path]::GetFullPath($runningProcess.ExecutablePath) -eq $backendExecutable) {
            $isServeFlowProcess = $true
        }
        if ($process.name -eq 'frontend' -and $nodePath -and $runningProcess.ExecutablePath -and [System.IO.Path]::GetFullPath($runningProcess.ExecutablePath) -eq [System.IO.Path]::GetFullPath($nodePath) -and $runningProcess.CommandLine.Contains($viteScript)) {
            $isServeFlowProcess = $true
        }
        if ($isServeFlowProcess) {
            Stop-Process -Id $process.id -ErrorAction Stop
        } else {
            Write-Warning "Skipped PID $($process.id) because it no longer matches the recorded ServeFlow process."
        }
    }
    Remove-Item -LiteralPath $stateFile -Force
}

Push-Location $repoRoot
try {
    & docker compose stop postgres redis
    if ($LASTEXITCODE -ne 0) {
        throw 'Docker Compose could not stop the ServeFlow PostgreSQL and Redis services.'
    }
} finally {
    Pop-Location
}

Write-Output 'ServeFlow backend, frontend, PostgreSQL, and Redis have been stopped.'
