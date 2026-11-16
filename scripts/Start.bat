@echo off
setlocal
cd /d "%~dp0.."
set "MODE=%~1"
if "%MODE%"=="gateway" goto run
if "%MODE%"=="account" goto run
if "%MODE%"=="game" goto run
if "%MODE%"=="world" goto run
echo Usage: Start.bat gateway^|account^|game^|world [config.json]
exit /b 1
:run
if "%~2"=="" (
  "bin\%MODE%.exe"
) else (
  "bin\%MODE%.exe" -config "%~2"
)
exit /b %errorlevel%
