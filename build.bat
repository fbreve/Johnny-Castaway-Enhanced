@echo off
echo ============================================
echo  Johnny Castaway 2026 - Windows Build
echo ============================================
echo.

where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 set "PATH=C:\Program Files\Go\bin;%PATH%"
set "PATH=D:\Tools\w64devkit-1.22.0\bin;%PATH%"

echo [1/3] Building...
for /f "usebackq tokens=*" %%a in (`powershell -Command "Get-Date -Format 'yyyy-MM-dd HH:mm:ss'"`) do set BUILD_TIME=%%a
go build -ldflags "-H windowsgui -X 'main.buildTime=%BUILD_TIME%'" -o JohnnyCastaway.exe .
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
