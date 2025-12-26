$proc = Get-Process -Name "binance-trading-bot" -ErrorAction SilentlyContinue

if ($proc) {
    Write-Host "Server is running" -ForegroundColor Green
    Write-Host "Process: $($proc.Name) | ID: $($proc.Id)"
}
else {
    Write-Host "Server is NOT running!" -ForegroundColor Red
    Write-Host "Starting server..." -ForegroundColor Yellow
    Start-Process "D:\Apps\binance-trading-bot\binance-trading-bot.exe" -NoNewWindow
    Start-Sleep -Seconds 5
    Write-Host "Server started. Waiting for API to be ready..." -ForegroundColor Green
}
