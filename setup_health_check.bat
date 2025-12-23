@echo off
REM Setup Health Check Scheduled Task
REM This script creates a Windows Task Scheduler task that runs the health check every 30 minutes

echo Setting up Binance Trading Bot Health Check...

REM Delete existing task if it exists
schtasks /delete /tn "BinanceBotHealthCheck" /f 2>nul

REM Create new task that runs every 30 minutes
schtasks /create /tn "BinanceBotHealthCheck" /tr "powershell.exe -ExecutionPolicy Bypass -File D:\Apps\binance-trading-bot\health_check.ps1" /sc minute /mo 30 /ru "%USERNAME%" /rl HIGHEST

if %ERRORLEVEL% EQU 0 (
    echo.
    echo SUCCESS: Health check task created!
    echo Task Name: BinanceBotHealthCheck
    echo Schedule: Every 30 minutes
    echo.
    echo To view/modify: Task Scheduler ^> BinanceBotHealthCheck
    echo To run manually: schtasks /run /tn "BinanceBotHealthCheck"
    echo To delete: schtasks /delete /tn "BinanceBotHealthCheck" /f
) else (
    echo.
    echo ERROR: Failed to create task. Try running as Administrator.
)

pause
