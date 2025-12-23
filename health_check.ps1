# Binance Trading Bot Health Check Script
# Runs every 30 minutes to ensure server and Ginie are running in live mode

$ServerPath = "D:\Apps\binance-trading-bot"
$ServerExe = "binance-trading-bot.exe"
$ServerPort = 8088
$ApiBaseUrl = "http://localhost:$ServerPort"
$LogFile = "$ServerPath\health_check.log"

function Write-Log {
    param([string]$Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] $Message"
    Add-Content -Path $LogFile -Value $logMessage
    Write-Host $logMessage
}

function Test-ServerRunning {
    # Check if process is running
    $process = Get-Process -Name "binance-trading-bot" -ErrorAction SilentlyContinue
    if ($null -eq $process) {
        return $false
    }

    # Also check if port is listening
    $connection = Test-NetConnection -ComputerName localhost -Port $ServerPort -WarningAction SilentlyContinue
    return $connection.TcpTestSucceeded
}

function Start-Server {
    Write-Log "Starting server..."
    Set-Location $ServerPath
    Start-Process -FilePath "$ServerPath\$ServerExe" -WindowStyle Hidden

    # Wait for server to start (max 30 seconds)
    $maxWait = 30
    $waited = 0
    while ($waited -lt $maxWait) {
        Start-Sleep -Seconds 2
        $waited += 2
        if (Test-ServerRunning) {
            Write-Log "Server started successfully after $waited seconds"
            # Give API a moment to initialize
            Start-Sleep -Seconds 3
            return $true
        }
    }

    Write-Log "ERROR: Server failed to start within $maxWait seconds"
    return $false
}

function Get-GinieStatus {
    try {
        $response = Invoke-RestMethod -Uri "$ApiBaseUrl/api/futures/ginie/autopilot/status" -Method GET -TimeoutSec 10
        return $response
    } catch {
        Write-Log "ERROR: Failed to get Ginie status: $_"
        return $null
    }
}

function Start-GinieAutopilot {
    try {
        $response = Invoke-RestMethod -Uri "$ApiBaseUrl/api/futures/ginie/autopilot/start" -Method POST -TimeoutSec 10
        return $response
    } catch {
        Write-Log "ERROR: Failed to start Ginie: $_"
        return $null
    }
}

function Get-GinieConfig {
    try {
        $response = Invoke-RestMethod -Uri "$ApiBaseUrl/api/futures/ginie/autopilot/config" -Method GET -TimeoutSec 10
        return $response
    } catch {
        Write-Log "ERROR: Failed to get Ginie config: $_"
        return $null
    }
}

# Main health check logic
Write-Log "========== Health Check Started =========="

# Step 1: Check if server is running
$serverRunning = Test-ServerRunning

if (-not $serverRunning) {
    Write-Log "Server is NOT running. Attempting to start..."
    $started = Start-Server
    if (-not $started) {
        Write-Log "CRITICAL: Could not start server. Exiting."
        exit 1
    }
} else {
    Write-Log "Server is running."
}

# Step 2: Check Ginie status
$ginieStatus = Get-GinieStatus

if ($null -eq $ginieStatus) {
    Write-Log "Could not retrieve Ginie status. Server may still be initializing."
    exit 1
}

$isGinieRunning = $ginieStatus.running -eq $true
$isLiveMode = $ginieStatus.config.dry_run -eq $false

Write-Log "Ginie Status: Running=$isGinieRunning, LiveMode=$isLiveMode"

# Step 3: If Ginie is not running but should be in live mode, start it
if (-not $isGinieRunning) {
    # Check if Ginie should be in live mode (dry_run = false means live mode)
    $config = Get-GinieConfig

    if ($null -ne $config -and $config.config.dry_run -eq $false) {
        Write-Log "Ginie is configured for LIVE mode but not running. Starting..."
        $startResult = Start-GinieAutopilot

        if ($null -ne $startResult -and $startResult.success -eq $true) {
            Write-Log "Ginie started successfully in LIVE mode"
        } else {
            Write-Log "ERROR: Failed to start Ginie"
        }
    } else {
        Write-Log "Ginie is in PAPER mode or config unavailable. No action needed."
    }
} else {
    if ($isLiveMode) {
        Write-Log "Ginie is running in LIVE mode. All good!"
    } else {
        Write-Log "Ginie is running in PAPER mode. No action needed."
    }
}

Write-Log "========== Health Check Completed =========="
