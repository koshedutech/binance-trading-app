@echo off
cd /d D:\Apps\binance-trading-bot
for /f "tokens=1,2 delims==" %%a in (.env) do set %%a=%%b
go run cmd/analyze_trades/main.go
pause
