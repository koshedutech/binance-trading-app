#!/usr/bin/env pwsh

Write-Host "=== Testing Custom ROI Target Position Closing ===" -ForegroundColor Cyan

# Get current positions
Write-Host "`nFetching current positions..." -ForegroundColor Yellow
$response = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/autopilot/positions" -TimeoutSec 10
$positions = $response.Content | ConvertFrom-Json

Write-Host "Found $($positions.count) positions`n" -ForegroundColor Green

# Find a position that's winning (positive unrealized PnL)
$winningPositions = $positions.positions | Where-Object { $_.unrealized_pnl -gt 0 } | Sort-Object unrealized_pnl -Descending

if ($winningPositions.Count -eq 0) {
    Write-Host "No winning positions found. Looking for positions closest to breakeven..." -ForegroundColor Yellow
    $targetPos = $positions.positions | Sort-Object { [Math]::Abs($_.unrealized_pnl) } | Select-Object -First 1
} else {
    $targetPos = $winningPositions[0]
}

Write-Host "Target Position: $($targetPos.symbol)" -ForegroundColor Cyan
Write-Host "  Entry Price: $($targetPos.entry_price)"
Write-Host "  Quantity: $($targetPos.original_qty)"
Write-Host "  Leverage: $($targetPos.leverage)"
Write-Host "  Side: $($targetPos.side)"
Write-Host "  Unrealized PnL: $($targetPos.unrealized_pnl) USD"

# Calculate entry value and current ROI
$entryValue = [Double]$targetPos.entry_price * [Double]$targetPos.original_qty
$marginUsed = $entryValue / [Double]$targetPos.leverage
$roiPercent = ([Double]$targetPos.unrealized_pnl / $marginUsed) * 100

Write-Host "  Entry Value: $entryValue USD"
Write-Host "  Margin Used: $marginUsed USD"
Write-Host "  Current ROI: $($roiPercent.ToString('F2'))%"

# Set a custom ROI target that's slightly above current ROI
$customRoi = [Math]::Ceiling($roiPercent) + 0.5
if ($customRoi -lt 0.5) { $customRoi = 0.5 }

Write-Host "`nSetting custom ROI target to: $customRoi%" -ForegroundColor Yellow

# Call the API to set custom ROI
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
    Write-Host "  Save for Future: $($setResult.save_for_future)"
} else {
    Write-Host "✗ Failed to set ROI target" -ForegroundColor Red
    Write-Host $setResult
    exit 1
}

Write-Host "`n=== Monitoring Position for ROI Target Trigger ===" -ForegroundColor Cyan
Write-Host "Monitoring $($targetPos.symbol) to close at $customRoi% ROI...`n" -ForegroundColor Yellow

$startTime = Get-Date
$checkInterval = 5
$maxDuration = 300

for ($i = 0; $i -lt ($maxDuration / $checkInterval); $i++) {
    Start-Sleep -Seconds $checkInterval

    # Check position status
    $checkResponse = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/autopilot/positions" -TimeoutSec 10
    $allPositions = $checkResponse.Content | ConvertFrom-Json

    $currentPos = $allPositions.positions | Where-Object { $_.symbol -eq $targetPos.symbol }

    if ($null -eq $currentPos) {
        Write-Host "✓ POSITION CLOSED!" -ForegroundColor Green
        $elapsedSeconds = ((Get-Date) - $startTime).TotalSeconds
        Write-Host "  Position $($targetPos.symbol) was closed after $([Math]::Round($elapsedSeconds, 1)) seconds"
        Write-Host "  Target ROI: $customRoi%"
        Write-Host "`nSUCCESS: Custom ROI target triggered position closing!" -ForegroundColor Green
        exit 0
    }

    $currentRoi = ([Double]$currentPos.unrealized_pnl / $marginUsed) * 100
    $elapsed = ((Get-Date) - $startTime).TotalSeconds

    Write-Host "[$([Math]::Round($elapsed, 1))s] $($targetPos.symbol): ROI=$($currentRoi.ToString('F2'))% (Target: $customRoi%)" -ForegroundColor Cyan

    if ($currentRoi -ge ($customRoi * 0.95)) {
        Write-Host "  ⚠ ROI approaching target! ($($currentRoi.ToString('F2'))% of $customRoi%)" -ForegroundColor Yellow
    }
}

Write-Host "`n⚠ Test timed out after $maxDuration seconds" -ForegroundColor Yellow
Write-Host "Position $($targetPos.symbol) did not close within the time limit" -ForegroundColor Yellow
