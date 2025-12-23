@echo off
REM ============================================================================
REM Binance Trading Bot - Build Script for Windows
REM ============================================================================

setlocal enabledelayedexpansion

echo ========================================
echo  Binance Trading Bot - Build Script
echo ========================================

REM Get script directory
set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%.."

REM Change to project root
cd /d "%PROJECT_ROOT%"

echo Detected: windows/amd64

REM Build version from git (if available)
for /f "tokens=*" %%i in ('git describe --tags --always 2^>nul') do set VERSION=%%i
if "%VERSION%"=="" set VERSION=dev

for /f "tokens=*" %%i in ('powershell -Command "Get-Date -Format 'yyyy-MM-dd_HH:mm:ss'"') do set BUILD_TIME=%%i

echo Version: %VERSION%
echo Build Time: %BUILD_TIME%

REM Create dist directory
set "DIST_DIR=%PROJECT_ROOT%\dist"
if not exist "%DIST_DIR%" mkdir "%DIST_DIR%"

REM Build frontend first
echo.
echo Building frontend...
cd /d "%PROJECT_ROOT%\web"
if exist "package.json" (
    call npm install
    call npm run build
    echo Frontend built successfully!
) else (
    echo Warning: Frontend package.json not found
)
cd /d "%PROJECT_ROOT%"

REM Build backend
echo.
echo Building backend for windows/amd64...

set OUTPUT_NAME=trading-bot.exe
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64

go build -ldflags "-X main.Version=%VERSION% -X main.BuildTime=%BUILD_TIME%" -o "%DIST_DIR%\%OUTPUT_NAME%" .

if %ERRORLEVEL% NEQ 0 (
    echo Build failed!
    exit /b 1
)

echo Backend built successfully!

REM Copy required files
echo.
echo Copying distribution files...
if exist "%PROJECT_ROOT%\web\dist" (
    xcopy /E /I /Y "%PROJECT_ROOT%\web\dist" "%DIST_DIR%\web\dist"
) else (
    mkdir "%DIST_DIR%\web\dist"
)
if exist "%PROJECT_ROOT%\.env.example" copy /Y "%PROJECT_ROOT%\.env.example" "%DIST_DIR%\"
if exist "%PROJECT_ROOT%\config.json.example" copy /Y "%PROJECT_ROOT%\config.json.example" "%DIST_DIR%\"

REM Create start script
(
echo @echo off
echo REM Start the trading bot
echo.
echo REM Check for .env file
echo if not exist ".env" ^(
echo     echo Warning: .env file not found!
echo     echo Please copy .env.example to .env and configure it.
echo     exit /b 1
echo ^)
echo.
echo REM Start the bot
echo trading-bot.exe
) > "%DIST_DIR%\start.bat"

echo.
echo ========================================
echo  Build Complete!
echo ========================================
echo Output: %DIST_DIR%
echo Binary: %OUTPUT_NAME%
echo.
echo To run:
echo   1. cd %DIST_DIR%
echo   2. copy .env.example .env
echo   3. Edit .env with your settings
echo   4. start.bat

endlocal
