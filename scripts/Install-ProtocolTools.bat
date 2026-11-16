@echo off
setlocal
if "%~1"=="" goto usage
if not "%~2"=="" goto usage
set "PROTOC_SOURCE=%~f1"
cd /d "%~dp0.." || exit /b 1
if not defined GO_EXE set "GO_EXE=go"
"%GO_EXE%" run github.com/haihhhhh/ht_server_frame/tools/protocol install -protoc-source "%PROTOC_SOURCE%"
exit /b %errorlevel%

:usage
echo 用法：Install-ProtocolTools.bat "protoc.exe 文件路径"
exit /b 1
