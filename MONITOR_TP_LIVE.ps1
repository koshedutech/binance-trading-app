# Real-Time TP Hit Monitor
# Tracks live positions and alerts when TPs are hit
# Run: powershell -ExecutionPolicy Bypass -File MONITOR_TP_LIVE.ps1

param(
    [int]$Interval = 10  # Check every 10 seconds
)

$script:lastCheck = @{}
$script:tpHits = @{}
$script:positions = @{}

# Colors for output
$colors = @{
    "header" = "Cyan"
    "position_open" = "Green"
    "tp_hit" = "Yellow"
    "tp_placed" = "Green"
    "tp_pending" = "Gray"
    "error" = "Red"
    "info" = "Gray"
}

# Position tracking
$positions = @{
    "AVNTUSDT" = @{symbol="AVNTUSDT"; side="LONG"; tp1=0.41; tp2=0.42; tp3=0.44; tp4=0.46; tpHits=@()}
    "LABUSDT" = @{symbol="LABUSDT"; side="SHORT"; tp1=0.14; tp2=0.14; tp3=0.13; tp4=0.12; tpHits=@()}
    "BNBUSDT" = @{symbol="BNBUSDT"; side="SHORT"; tp1=816.11; tp2=790.87; tp3=757.22; tp4=715.15; tpHits=@()}
    "USELESSUSDT" = @{symbol="USELESSUSDT"; side="LONG"; tp1=0.06; tp2=0.07; tp3=0.07; tp4=0.07; tpHits=@()}
    "MIRAUSDT" = @{symbol="MIRAUSDT"; side="LONG"; tp1=0.14; tp2=0.15; tp3=0.15; tp4=0.16; tpHits=@()}
}

function Write-Header {
    Clear-Host
    Write-Host "╔════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "║         GINIE TP HIT LIVE MONITORING - ACTIVE              ║" -ForegroundColor Cyan
    Write-Host "╚════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Server: http://localhost:8094" -ForegroundColor Gray
    Write-Host "Checking every $Interval seconds" -ForegroundColor Gray
    Write-Host "Press Ctrl+C to stop" -ForegroundColor Yellow
    Write-Host ""
}

function Get-Positions {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/autopilot/status" `
            -TimeoutSec 5 -ErrorAction SilentlyContinue

        if ($response.StatusCode -eq 200) {
            $data = $response.Content | ConvertFrom-Json
            return $data.positions
        }
    }
    catch {
        return $null
    }
}

function Get-ServerLogs {
    $logFile = "D:\Apps\binance-trading-bot\server.log"

    if (Test-Path $logFile) {
        try {
            $content = Get-Content $logFile -Tail 100 -ErrorAction SilentlyContinue
            return $content
        }
        catch {
            return $null
        }
    }
    return $null
}

function Check-TPHits {
    param([object]$positions)

    if (!$positions) {
        Write-Host "No positions data available" -ForegroundColor Red
        return
    }

    foreach ($pos in $positions) {
        $symbol = $pos.symbol
        Write-Host ""
        Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray
        Write-Host "Position: $symbol - $(if($pos.side -eq 'LONG') {Write-Host 'LONG ↑' -ForegroundColor Green -NoNewline} else {Write-Host 'SHORT ↓' -ForegroundColor Red -NoNewline})" -ForegroundColor White
        Write-Host " | Mode: $($pos.mode)" -ForegroundColor Gray
        Write-Host ""

        # Display entry and current info
        Write-Host "  Entry: `$$($pos.entry_price | ForEach-Object {[math]::Round($_, 8)})" -ForegroundColor Gray
        Write-Host "  Qty: $($pos.original_qty) → $($pos.remaining_qty) remaining" -ForegroundColor Gray
        Write-Host "  UnrealizedPnL: `$$('{0:F2}' -f $pos.unrealized_pnl)" -ForegroundColor $(if($pos.unrealized_pnl -ge 0) {"Green"} else {"Red"})
        Write-Host "  RealizedPnL: `$$('{0:F2}' -f $pos.realized_pnl)" -ForegroundColor $(if($pos.realized_pnl -ge 0) {"Green"} else {"Red"})
        Write-Host ""

        # Display TP progression
        Write-Host "  TP Progression:" -ForegroundColor Cyan
        Write-Host "  " -NoNewline

        for ($i = 0; $i -lt $pos.take_profits.Count; $i++) {
            $tp = $pos.take_profits[$i]

            if ($tp.status -eq "hit") {
                Write-Host "[TP$($tp.level) ✓]" -ForegroundColor Green -NoNewline
            }
            elseif ($pos.current_tp_level + 1 -eq $tp.level) {
                Write-Host "[TP$($tp.level) ⚠]" -ForegroundColor Yellow -NoNewline
            }
            else {
                Write-Host "[TP$($tp.level) ○]" -ForegroundColor Gray -NoNewline
            }

            if ($i -lt $pos.take_profits.Count - 1) {
                Write-Host " → " -ForegroundColor Gray -NoNewline
            }
        }
        Write-Host ""
        Write-Host ""

        # Display TP details
        Write-Host "  TP Details:" -ForegroundColor Cyan
        Write-Host "  " -NoNewline

        foreach ($tp in $pos.take_profits) {
            $statusColor = if ($tp.status -eq "hit") {"Green"} elseif ($pos.current_tp_level + 1 -eq $tp.level) {"Yellow"} else {"Gray"}
            Write-Host "[$($tp.level):`$$($tp.price | ForEach-Object {[math]::Round($_, 8)})/$($tp.percent)%]" -ForegroundColor $statusColor -NoNewline
            Write-Host " " -NoNewline
        }
        Write-Host ""
        Write-Host ""

        # Check for TP hits
        if ($pos.current_tp_level -gt 0) {
            Write-Host "  ✓ TP$($pos.current_tp_level) HIT! | $($pos.current_tp_level) of 4 levels completed" -ForegroundColor Green
        }
    }
}

function Check-LogsForTPEvents {
    $logs = Get-ServerLogs

    if ($logs) {
        Write-Host ""
        Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray
        Write-Host "RECENT LOG EVENTS:" -ForegroundColor Cyan
        Write-Host ""

        $tpEvents = @()

        foreach ($line in $logs) {
            if ($line -match "TP level hit|placeNextTPOrder|Next take profit|Failed to place" -and $line -match "INFO|ERROR") {
                $tpEvents += $line
            }
        }

        if ($tpEvents.Count -gt 0) {
            $tpEvents | Select-Object -Last 10 | ForEach-Object {
                if ($_ -match "TP level hit") {
                    Write-Host "[TP HIT] $_" -ForegroundColor Yellow
                }
                elseif ($_ -match "Next take profit order placed") {
                    Write-Host "[SUCCESS] $_" -ForegroundColor Green
                }
                elseif ($_ -match "placeNextTPOrder called") {
                    Write-Host "[FUNCTION] $_" -ForegroundColor Cyan
                }
                elseif ($_ -match "Failed") {
                    Write-Host "[ERROR] $_" -ForegroundColor Red
                }
                else {
                    Write-Host "[LOG] $_" -ForegroundColor Gray
                }
            }
        }
        else {
            Write-Host "No TP events logged yet - waiting for prices to reach TP levels..." -ForegroundColor Gray
        }
    }
}

function Show-Summary {
    Write-Host ""
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray
    Write-Host "SUMMARY:" -ForegroundColor Cyan

    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/autopilot/status" `
            -TimeoutSec 5 -ErrorAction SilentlyContinue

        if ($response.StatusCode -eq 200) {
            $data = $response.Content | ConvertFrom-Json
            $stats = $data.stats

            Write-Host "  Active Positions: $($stats.active_positions) / $($stats.max_positions)" -ForegroundColor Cyan
            Write-Host "  Unrealized PnL: `$$('{0:F2}' -f $stats.combined_pnl)" -ForegroundColor $(if($stats.combined_pnl -ge 0) {"Green"} else {"Red"})
            Write-Host "  Daily PnL: `$$('{0:F2}' -f $stats.daily_pnl)" -ForegroundColor $(if($stats.daily_pnl -ge 0) {"Green"} else {"Red"})
            Write-Host "  Total PnL: `$$('{0:F2}' -f $stats.total_pnl)" -ForegroundColor Green
            Write-Host "  Win Rate: $($stats.win_rate)%" -ForegroundColor Cyan
            Write-Host "  Dry Run: $(if($stats.dry_run) {'ON (Paper Mode)'} else {'OFF (Live Mode)'})" -ForegroundColor $(if($stats.dry_run) {"Yellow"} else {"Green"})
        }
    }
    catch {
        Write-Host "  Could not fetch summary" -ForegroundColor Red
    }

    Write-Host ""
    Write-Host "Last update: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
    Write-Host ""
}

# Main monitoring loop
Write-Host "Starting monitoring..." -ForegroundColor Yellow
Start-Sleep -Seconds 2

while ($true) {
    Write-Header

    $positions = Get-Positions

    if ($positions) {
        Check-TPHits $positions
        Check-LogsForTPEvents
        Show-Summary

        Write-Host "Next check in $Interval seconds..." -ForegroundColor Gray
    }
    else {
        Write-Host "ERROR: Could not connect to server" -ForegroundColor Red
        Write-Host "Make sure the server is running: ./binance-trading-bot.exe" -ForegroundColor Yellow
    }

    for ($i = 0; $i -lt $Interval; $i++) {
        Write-Host "`rWaiting... ($($Interval - $i)s)" -ForegroundColor Gray -NoNewline
        Start-Sleep -Seconds 1
    }
}
