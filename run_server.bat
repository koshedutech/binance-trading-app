@echo off
echo Checking Go version...
"C:\Program Files\Go\bin\go.exe" version
echo.
echo Starting server...
cd /d D:\Apps\binance-trading-bot
"C:\Program Files\Go\bin\go.exe" run . 2>&1
pause
