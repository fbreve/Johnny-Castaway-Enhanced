@echo off
echo ============================================
echo  Johnny Castaway 2026 - Windows Build
echo ============================================
echo.

set PATH=C:\mingw64\bin;%PATH%

echo [1/3] Building...
go build -ldflags "-H windowsgui" -o JohnnyCastaway.exe .
if %ERRORLEVEL% NEQ 0 (
    echo BUILD FAILED
    pause
    exit /b 1
)

echo [2/3] Creating screensaver...
copy /Y JohnnyCastaway.exe JohnnyCastaway.scr >nul

echo [3/3] Done!
echo.
echo Output:
echo   JohnnyCastaway.exe  - run as desktop app
echo   JohnnyCastaway.scr  - right-click, Install as screensaver
echo.
echo Everything is embedded in the exe. No external files needed.
echo.
pause
