@echo off
setlocal
cd /d "%~dp0.."
if not defined GO_EXE set "GO_EXE=go"
"%GO_EXE%" run ./tools/moduleimports
if errorlevel 1 exit /b 1
if not exist bin mkdir bin
for %%s in (gateway account game world client) do (
  "%GO_EXE%" build -trimpath -o "bin\%%s.exe" "./cmd/%%s"
  if errorlevel 1 exit /b 1
)
"%GO_EXE%" vet ./...
exit /b %errorlevel%
