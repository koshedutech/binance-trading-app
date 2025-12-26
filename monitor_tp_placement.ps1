# Real-Time TP Placement Monitor
# This script monitors the server logs for multi-level TP placement
# Run: powershell -ExecutionPolicy Bypass -File monitor_tp_placement.ps1

$logFile = "D:\Apps\binance-trading-bot\server.log"
$colors = @{
    "position_opened" = "Green"
    "tp_level_hit" = "Yellow"
    "place_next_tp" = "Cyan"
    "tp_placed" = "Green"
    "tp_failed" = "Red"
    "TP1" = "Yellow"
    "TP2" = "Cyan"
    "TP3" = "Magenta"
    "TP4" = "Green"
}

Write-Host "==================================" -ForegroundColor Cyan
Write-Host "TP Placement Real-Time Monitor" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan
Write-Host "Monitoring: $logFile" -ForegroundColor Gray
Write-Host "Press Ctrl+C to stop" -ForegroundColor Gray
Write-Host "" -ForegroundColor Gray

# Get initial file size to start from end
$lastSize = (Get-Item $logFile).Length
$lastCheckTime = Get-Date

# Color output function
function Write-LogLine {
    param(
        [string]$line,
        [string]$keyword
    )

    if ($line -match "TP level hit - placing next TP order") {
        Write-Host "[TP-HIT] $line" -ForegroundColor Yellow
    }
    elseif ($line -match "placeNextTPOrder called") {
        Write-Host "[FUNC-CALL] $line" -ForegroundColor Cyan
    }
    elseif ($line -match "Next take profit order placed") {
        Write-Host "[TP-SUCCESS] $line" -ForegroundColor Green
    }
    elseif ($line -match "Failed to place next take profit") {
        Write-Host "[TP-ERROR] $line" -ForegroundColor Red
    }
    elseif ($line -match "Created new Ginie position|Ginie position opened") {
        Write-Host "[POSITION-OPEN] $line" -ForegroundColor Green
    }
    elseif ($line -match "tp_level_hit|tp2|tp3|tp4" -and $line -match "INFO") {
        Write-Host "[TP-INFO] $line" -ForegroundColor Magenta
    }
}

# Main monitoring loop
while ($true) {
    try {
        if (Test-Path $logFile) {
            $currentSize = (Get-Item $logFile).Length

            # If file is larger than before, read new lines
            if ($currentSize -gt $lastSize) {
                $newContent = Get-Content $logFile | Select-Object -Last 50

                foreach ($line in $newContent) {
                    # Check for TP-related keywords
                    if ($line -match "TP level hit|placeNextTPOrder|Next take profit|Failed to place next|Ginie position|tp_level|tp2|tp3|tp4") {
                        Write-LogLine $line
                    }
                }

                $lastSize = $currentSize
            }
        }

        # Also check API status periodically
        $currentTime = Get-Date
        if (($currentTime - $lastCheckTime).TotalSeconds -gt 30) {
            try {
                $health = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/diagnostics" `
                    -TimeoutSec 3 -ErrorAction SilentlyContinue

                if ($health.StatusCode -eq 200) {
                    $content = $health.Content | ConvertFrom-Json

                    $positions = $content.positions.open_count
                    $tpHits = $content.profit_booking.tp_hits_last_hour
                    $partialCloses = $content.profit_booking.partial_closes_last_hour

                    Write-Host "[API-CHECK] Positions: $positions | TP Hits (1h): $tpHits | Partial Closes (1h): $partialCloses" -ForegroundColor Gray
                }
            }
            catch {
                # Silent - API might not be available
            }

            $lastCheckTime = $currentTime
        }

        Start-Sleep -Milliseconds 500
    }
    catch {
        Write-Host "Error: $_" -ForegroundColor Red
        Start-Sleep -Seconds 1
    }
}
