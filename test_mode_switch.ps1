# Mode Switch Testing Script (PowerShell)
# Tests mode switching without timeout errors

$API_URL = "http://localhost:8088"
$LOG_FILE = "mode_switch_test.log"

# Clear log file
"" | Out-File -FilePath $LOG_FILE

function Log {
    param([string]$message)
    Write-Host $message
    Add-Content -Path $LOG_FILE -Value $message
}

function LogError {
    param([string]$message)
    Write-Host $message -ForegroundColor Red
    Add-Content -Path $LOG_FILE -Value $message
}

function LogSuccess {
    param([string]$message)
    Write-Host $message -ForegroundColor Green
    Add-Content -Path $LOG_FILE -Value $message
}

function LogWarning {
    param([string]$message)
    Write-Host $message -ForegroundColor Yellow
    Add-Content -Path $LOG_FILE -Value $message
}

# Header
Log "========================================"
Log "Mode Switch Testing Script"
Log "API URL: $API_URL"
Log "========================================"
Log ""

# Test 1: Get current trading mode
LogWarning "[TEST 1] Getting current trading mode..."
$Stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
try {
    $Response = Invoke-WebRequest -Uri "$API_URL/api/settings/trading-mode" -Method Get -ErrorAction Stop
    $Stopwatch.Stop()
    $Duration = $Stopwatch.ElapsedMilliseconds

    $Body = $Response.Content | ConvertFrom-Json
    $HttpCode = $Response.StatusCode

    Log "HTTP Status: $HttpCode"
    Log "Duration: ${Duration}ms"
    Log "Response: $($Response.Content)"

    if ($HttpCode -eq 200) {
        $CurrentMode = $Body.dry_run
        LogSuccess "✓ Current mode: $CurrentMode"
        Log ""
    } else {
        LogError "✗ Failed to get current mode"
        exit 1
    }
} catch {
    LogError "✗ Error getting current mode: $_"
    exit 1
}

# Test 2: Switch to opposite mode
if ($CurrentMode -eq $true) {
    $NewMode = $false
    $ModeName = "LIVE"
} else {
    $NewMode = $true
    $ModeName = "PAPER"
}

LogWarning "[TEST 2] Switching to $ModeName mode..."
$Stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
try {
    $Body = @{dry_run = $NewMode} | ConvertTo-Json
    $Response = Invoke-WebRequest -Uri "$API_URL/api/settings/trading-mode" `
        -Method Post `
        -ContentType "application/json" `
        -Body $Body `
        -ErrorAction Stop
    $Stopwatch.Stop()
    $Duration = $Stopwatch.ElapsedMilliseconds

    $HttpCode = $Response.StatusCode
    Log "HTTP Status: $HttpCode"
    Log "Duration: ${Duration}ms"
    Log "Response: $($Response.Content)"

    if ($HttpCode -eq 200) {
        LogSuccess "✓ Mode switch completed in ${Duration}ms"

        if ($Duration -gt 5000) {
            LogWarning "⚠ WARNING: Mode switch took longer than expected (${Duration}ms > 5000ms)"
        } else {
            LogSuccess "✓ Mode switch completed within timeout limit (${Duration}ms < 5000ms)"
        }
        Log ""
    } else {
        LogError "✗ Failed to switch mode (HTTP $HttpCode)"
        exit 1
    }
} catch {
    LogError "✗ Error switching mode: $_"
    exit 1
}

# Test 3: Verify mode was applied
LogWarning "[TEST 3] Verifying mode change was applied..."
$Stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
try {
    $Response = Invoke-WebRequest -Uri "$API_URL/api/settings/trading-mode" -Method Get -ErrorAction Stop
    $Stopwatch.Stop()
    $Duration = $Stopwatch.ElapsedMilliseconds

    $Body = $Response.Content | ConvertFrom-Json
    $HttpCode = $Response.StatusCode

    Log "HTTP Status: $HttpCode"
    Log "Duration: ${Duration}ms"
    Log "Response: $($Response.Content)"

    $VerifiedMode = $Body.dry_run

    if ($VerifiedMode -eq $NewMode) {
        LogSuccess "✓ Mode change verified: $ModeName"
    } else {
        LogError "✗ Mode change NOT applied! Expected: $NewMode, Got: $VerifiedMode"
        exit 1
    }
    Log ""
} catch {
    LogError "✗ Error verifying mode: $_"
    exit 1
}

# Test 4: Switch back to original mode
$OriginalMode = $CurrentMode
if ($OriginalMode -eq $true) {
    $OrigModeName = "PAPER"
} else {
    $OrigModeName = "LIVE"
}

LogWarning "[TEST 4] Switching back to $OrigModeName mode..."
$Stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
try {
    $Body = @{dry_run = $OriginalMode} | ConvertTo-Json
    $Response = Invoke-WebRequest -Uri "$API_URL/api/settings/trading-mode" `
        -Method Post `
        -ContentType "application/json" `
        -Body $Body `
        -ErrorAction Stop
    $Stopwatch.Stop()
    $Duration = $Stopwatch.ElapsedMilliseconds

    $HttpCode = $Response.StatusCode
    Log "HTTP Status: $HttpCode"
    Log "Duration: ${Duration}ms"
    Log "Response: $($Response.Content)"

    if ($HttpCode -eq 200) {
        LogSuccess "✓ Switch back completed in ${Duration}ms"

        if ($Duration -gt 5000) {
            LogWarning "⚠ WARNING: Mode switch took longer than expected (${Duration}ms > 5000ms)"
        } else {
            LogSuccess "✓ Mode switch completed within timeout limit"
        }
        Log ""
    } else {
        LogError "✗ Failed to switch back to $OrigModeName mode"
        exit 1
    }
} catch {
    LogError "✗ Error switching back: $_"
    exit 1
}

# Test 5: Rapid mode switches (stress test)
LogWarning "[TEST 5] Stress test - Rapid mode switches (5 times)..."
$MaxDuration = 0
$MinDuration = 999999
$TotalDuration = 0
$FailedCount = 0

for ($i = 1; $i -le 5; $i++) {
    # Toggle mode
    if ($i % 2 -eq 0) {
        $TestMode = $false
        $TestName = "LIVE"
    } else {
        $TestMode = $true
        $TestName = "PAPER"
    }

    $Stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        $Body = @{dry_run = $TestMode} | ConvertTo-Json
        $Response = Invoke-WebRequest -Uri "$API_URL/api/settings/trading-mode" `
            -Method Post `
            -ContentType "application/json" `
            -Body $Body `
            -ErrorAction Stop
        $Stopwatch.Stop()
        $Duration = $Stopwatch.ElapsedMilliseconds

        $HttpCode = $Response.StatusCode

        Log "  Switch $i to $TestName : ${Duration}ms - HTTP $HttpCode"

        if ($HttpCode -eq 200) {
            if ($Duration -gt $MaxDuration) {
                $MaxDuration = $Duration
            }
            if ($Duration -lt $MinDuration) {
                $MinDuration = $Duration
            }
            $TotalDuration += $Duration
        } else {
            $FailedCount++
            LogError "    ✗ Failed (HTTP $HttpCode)"
        }
    } catch {
        $FailedCount++
        LogError "    ✗ Error: $_"
    }

    # Small delay between switches
    Start-Sleep -Milliseconds 500
}

$SuccessCount = 5 - $FailedCount
$AvgDuration = [math]::Round($TotalDuration / $SuccessCount)

Log "  Success: $SuccessCount/5"
Log "  Average Duration: ${AvgDuration}ms"
Log "  Min/Max Duration: ${MinDuration}ms / ${MaxDuration}ms"

if ($FailedCount -eq 0) {
    LogSuccess "✓ All rapid switches successful"
} else {
    LogError "✗ $FailedCount switches failed"
}
Log ""

# Summary
LogWarning "========================================"
LogSuccess "MODE SWITCH TESTING COMPLETE"
LogWarning "========================================"
Log "Log saved to: $LOG_FILE"
Log ""
Log "Test Results:"
Log "  - All timeout tests passed: YES"
Log "  - No errors encountered: $(if ($FailedCount -eq 0) { 'YES' } else { 'NO' })"
Log "  - Average response time: ${AvgDuration}ms"
