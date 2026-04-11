@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul 2>nul

cd /d "%~dp0"

if "%VERSION%"=="" set VERSION=0.1.0

echo.
echo ========================================
echo   MYT Panel Build v%VERSION%
echo ========================================
echo.
echo   Supported devices:
echo     1. r1s
echo     2. r1q
echo     3. r1z
echo     4. c1
echo     5. q1
echo     6. q1n
echo     7. p1
echo     A. ALL (build all devices)
echo.

if not "%DEVICE%"=="" goto skip_menu
set /p "CHOICE=Select device [1-7/A]: "

if "%CHOICE%"=="1" set DEVICE=r1s
if "%CHOICE%"=="2" set DEVICE=r1q
if "%CHOICE%"=="3" set DEVICE=r1z
if "%CHOICE%"=="4" set DEVICE=c1
if "%CHOICE%"=="5" set DEVICE=q1
if "%CHOICE%"=="6" set DEVICE=q1n
if "%CHOICE%"=="7" set DEVICE=p1
if /i "%CHOICE%"=="A" goto build_all

if "%DEVICE%"=="" (
    echo Invalid choice.
    pause
    exit /b 1
)

:skip_menu
call :build_one %DEVICE%
goto done

:build_all
echo.
echo Building all devices...
echo.

call :build_frontend
if !errorlevel! neq 0 goto done

for %%d in (r1s r1q r1z c1 q1 q1n p1) do (
    call :build_backend %%d
)
goto done

:build_frontend
echo [Frontend] Building...
cd frontend
call npm install --silent 2>nul
if !errorlevel! neq 0 (
    echo ERROR: npm install failed
    cd ..
    exit /b 1
)
call npm run build
if !errorlevel! neq 0 (
    echo ERROR: frontend build failed
    cd ..
    exit /b 1
)
cd ..
echo [Frontend] OK
echo.
exit /b 0

:build_backend
set "DEV=%~1"
set "LDFL=-s -w -X main.Version=%VERSION% -X main.Device=%DEV%"
set "RDIR=release\%DEV%\v%VERSION%"

echo [%DEV%] Compiling...
set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
go build -trimpath -ldflags "%LDFL%" -o myt-panel .
if !errorlevel! neq 0 (
    echo [%DEV%] ERROR: build failed
    exit /b 1
)

set "SHA="
for /f "skip=1 tokens=*" %%a in ('certutil -hashfile myt-panel SHA256') do (
    if not defined SHA set "SHA=%%a"
)
echo %SHA%> myt-panel.sha256

if not exist "%RDIR%\deploy" mkdir "%RDIR%\deploy"
move /y myt-panel "%RDIR%\" >nul
move /y myt-panel.sha256 "%RDIR%\" >nul
echo v%VERSION% > "%RDIR%\VERSION"
copy /y deploy\alpine-openrc "%RDIR%\deploy\" >nul
copy /y deploy\debian-systemd.service "%RDIR%\deploy\" >nul
copy /y deploy\install-alpine.sh "%RDIR%\deploy\" >nul
copy /y deploy\install-debian.sh "%RDIR%\deploy\" >nul
copy /y deploy\README.txt "%RDIR%\" >nul

echo [%DEV%] OK  -  %SHA%
echo.
exit /b 0

:build_one
call :build_frontend
if !errorlevel! neq 0 goto done
call :build_backend %~1
goto done

:done
echo.
echo ========================================
echo   Build complete!  v%VERSION%
echo ========================================
echo.
echo   release\
for %%d in (r1s r1q r1z c1 q1 q1n p1) do (
    if exist "release\%%d\v%VERSION%\myt-panel" (
        echo     %%d\v%VERSION%\myt-panel
    )
)
echo.
pause
