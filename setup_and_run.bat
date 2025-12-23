@echo off
cd /d D:\Apps\binance-trading-bot
echo Downloading dependencies...
go mod tidy
echo.
echo Starting server...
go run .
pause
