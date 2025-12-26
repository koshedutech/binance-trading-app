Write-Host "=== Testing Custom ROI Target Position Closing ===" -ForegroundColor Cyan

# Get current positions
Write-Host "`nFetching current positions..." -ForegroundColor Yellow
$response = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/autopilot/positions" -TimeoutSec 10
$positions = $response.Content | ConvertFrom-Json

Write-Host "Found $($positions.count) positions`n" -ForegroundColor Green

# Find a position with positive ROI
$winningPos = $positions.positions | Where-Object { $_.unrealized_pnl -gt 0 } | Sort-Object unrealized_pnl -Descending | Select-Object -First 1

if ($null -eq $winningPos) {
    Write-Host "No winning positions found. Using AVAXUSDT which has positive unrealized PnL" -ForegroundColor Yellow
    $targetPos = $positions.positions | Where-Object { $_.symbol -eq "AVAXUSDT" }
} else {
    $targetPos = $winningPos
}

Write-Host "Target Position: $($targetPos.symbol)" -ForegroundColor Cyan
Write-Host "  Entry Price: $($targetPos.entry_price)"
Write-Host "  Quantity: $($targetPos.original_qty)"
Write-Host "  Leverage: $($targetPos.leverage)"
Write-Host "  Unrealized PnL: $($targetPos.unrealized_pnl) USD"

# Calculate ROI
$entryValue = [Double]$targetPos.entry_price * [Double]$targetPos.original_qty
$marginUsed = $entryValue / [Double]$targetPos.leverage
$currentRoi = ([Double]$targetPos.unrealized_pnl / $marginUsed) * 100

Write-Host "  Current ROI: $($currentRoi.ToString('F2'))%"

# Set custom ROI target slightly above current
$customRoi = if ($currentRoi -le 0) { 0.5 } else { $currentRoi + 0.5 }

Write-Host "`nSetting custom ROI target to: $customRoi%" -ForegroundColor Yellow

$apiRequest = @{
    roi_percent = $customRoi
    save_for_future = $false
} | ConvertTo-Json

$setResponse = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/positions/$($targetPos.symbol)/roi-target" `
    -Method POST `
    -Headers @{"Content-Type" = "application/json"} `
    -Body $apiRequest `
    -TimeoutSec 5

$setResult = $setResponse.Content | ConvertFrom-Json

if ($setResult.success) {
    Write-Host "✓ Custom ROI target set successfully!" -ForegroundColor Green
    Write-Host "  Symbol: $($setResult.symbol)"
    Write-Host "  ROI Target: $($setResult.roi_percent)%"
} else {
    Write-Host "✗ Failed to set ROI target" -ForegroundColor Red
    exit 1
}

Write-Host "`nMonitoring $($targetPos.symbol) for $customRoi% ROI closure (checking every 5 seconds)..." -ForegroundColor Yellow

$startTime = Get-Date

for ($i = 0; $i -lt 60; $i++) {
    Start-Sleep -Seconds 5
    
    $checkResponse = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/autopilot/positions" -TimeoutSec 10
    $allPositions = $checkResponse.Content | ConvertFrom-Json
    
    $currentPos = $allPositions.positions | Where-Object { $_.symbol -eq $targetPos.symbol }
    
    if ($null -eq $currentPos) {
        $elapsed = ((Get-Date) - $startTime).TotalSeconds
        Write-Host "`n✓ POSITION CLOSED!" -ForegroundColor Green
        Write-Host "  Closed after $([Math]::Round($elapsed, 1)) seconds"
        Write-Host "  Target ROI: $customRoi%"
        exit 0
    }
    
    $roi = ([Double]$currentPos.unrealized_pnl / $marginUsed) * 100
    $elapsed = ((Get-Date) - $startTime).TotalSeconds
    
    Write-Host "[$([Math]::Round($elapsed))s] ROI: $($roi.ToString('F2'))% (Target: $customRoi%)" -ForegroundColor Cyan
}

Write-Host "`nTest timed out after 5 minutes" -ForegroundColor Yellow
