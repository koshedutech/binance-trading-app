@echo off
REM Binance Trading Bot Health Check (Batch Version)
REM Checks server and Ginie status every 30 minutes

setlocal enabledelayedexpansion

set SERVER_PATH=D:\Apps\binance-trading-bot
set SERVER_EXE=binance-trading-bot.exe
set SERVER_PORT=8088
set API_URL=http://localhost:%SERVER_PORT%

cd /d %SERVER_PATH%

echo [%date% %time%] ========== Health Check Started ==========

REM Step 1: Check if server process is running
tasklist /FI "IMAGENAME eq %SERVER_EXE%" 2>NUL | find /I /N "%SERVER_EXE%">NUL
if "%ERRORLEVEL%"=="0" (
    echo [%date% %time%] Server process is running.
    goto :check_port
) else (
    echo [%date% %time%] Server is NOT running. Starting...
    goto :start_server
)

:check_port
REM Check if port is listening
netstat -ano | findstr ":%SERVER_PORT%" | findstr "LISTENING" >NUL
if "%ERRORLEVEL%"=="0" (
    echo [%date% %time%] Server port %SERVER_PORT% is listening.
    goto :check_ginie
) else (
    echo [%date% %time%] Server port not listening. Starting server...
    goto :start_server
)

:start_server
echo [%date% %time%] Starting server...
start /B cmd /c "%SERVER_PATH%\%SERVER_EXE% > %SERVER_PATH%\server.log 2>&1"

REM Wait for server to start
timeout /t 10 /nobreak >NUL

REM Verify server started
netstat -ano | findstr ":%SERVER_PORT%" | findstr "LISTENING" >NUL
if "%ERRORLEVEL%"=="0" (
    echo [%date% %time%] Server started successfully.
    timeout /t 5 /nobreak >NUL
    goto :check_ginie
) else (
    echo [%date% %time%] ERROR: Server failed to start.
    goto :end
)

:check_ginie
REM Check Ginie status via API using curl
echo [%date% %time%] Checking Ginie status...

curl -s -o ginie_status.json "%API_URL%/api/futures/ginie/autopilot/status" 2>NUL
if not exist ginie_status.json (
    echo [%date% %time%] ERROR: Could not get Ginie status.
    goto :end
)

REM Check if Ginie is running (look for "running":true)
findstr /C:"\"running\":true" ginie_status.json >NUL
if "%ERRORLEVEL%"=="0" (
    echo [%date% %time%] Ginie is RUNNING.

    REM Check if it's in live mode (look for "dry_run":false)
    findstr /C:"\"dry_run\":false" ginie_status.json >NUL
    if "!ERRORLEVEL!"=="0" (
        echo [%date% %time%] Ginie is in LIVE mode. All good!
    ) else (
        echo [%date% %time%] Ginie is in PAPER mode.
    )
    goto :cleanup
) else (
    echo [%date% %time%] Ginie is NOT running.
    goto :start_ginie
)

:start_ginie
REM Check if Ginie should be started (check settings file for live mode)
findstr /C:"\"ginie_dry_run_mode\":false" autopilot_settings.json >NUL
if "%ERRORLEVEL%"=="0" (
    echo [%date% %time%] Ginie is configured for LIVE mode. Starting...

    REM Start Ginie via API
    curl -s -X POST "%API_URL%/api/futures/ginie/autopilot/start" -o ginie_start.json 2>NUL

    findstr /C:"\"success\":true" ginie_start.json >NUL
    if "!ERRORLEVEL!"=="0" (
        echo [%date% %time%] Ginie started successfully in LIVE mode!
    ) else (
        echo [%date% %time%] ERROR: Failed to start Ginie.
    )
    del ginie_start.json 2>NUL
) else (
    echo [%date% %time%] Ginie is in PAPER mode or settings not found. No action.
)

:cleanup
del ginie_status.json 2>NUL

:end
echo [%date% %time%] ========== Health Check Completed ==========
echo.
