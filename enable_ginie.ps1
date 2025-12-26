# Enable Ginie Autopilot

$body = @{
    enabled = $true
} | ConvertTo-Json

$response = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/toggle" `
    -Method Post `
    -ContentType "application/json" `
    -Body $body `
    -TimeoutSec 5

$result = $response.Content | ConvertFrom-Json

Write-Host "Ginie Toggle Response:" -ForegroundColor Green
Write-Host $result | Format-List

Write-Host ""
Write-Host "Checking status..." -ForegroundColor Cyan
Start-Sleep -Seconds 2

$statusResponse = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/status" `
    -TimeoutSec 5

$status = $statusResponse.Content | ConvertFrom-Json

Write-Host "Ginie Status:" -ForegroundColor Green
Write-Host ("  Enabled: " + $status.enabled) -ForegroundColor (if($status.enabled) {"Green"} else {"Red"})
Write-Host ("  Active Mode: " + $status.active_mode) -ForegroundColor Cyan
Write-Host ("  Active Positions: " + $status.active_positions) -ForegroundColor Cyan
Write-Host ("  Last Decision: " + $status.last_decision_time) -ForegroundColor Gray
Write-Host ""
Write-Host "Ginie is now ENABLED and scanning for trading signals!" -ForegroundColor Green
Write-Host "Positions will open when high-confidence signals are found." -ForegroundColor Gray
Write-Host ""
Write-Host "Estimated wait for first position: 5-30 minutes" -ForegroundColor Yellow
