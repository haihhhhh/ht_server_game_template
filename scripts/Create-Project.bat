@echo off
setlocal
if "%~1"=="" goto usage
if not "%~2"=="" goto usage
cd /d "%~dp0.." || exit /b 1
if not defined GO_EXE set "GO_EXE=go"
"%GO_EXE%" run ./tools/newproject "%~1"
exit /b %errorlevel%

:usage
echo Usage: Create-Project ^<Go module^>
exit /b 1
