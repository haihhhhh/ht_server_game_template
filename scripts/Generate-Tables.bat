@echo off
setlocal
cd /d "%~dp0.."
if not defined GO_EXE set "GO_EXE=go"
if not defined DOTNET_EXE set "DOTNET_EXE=dotnet"
if not defined LUBAN_DLL set "LUBAN_DLL=%CD%\..\.tools\luban-5.1.0\Luban\Luban.dll"
set "GOWORK=off"
set "GAME_MODULE="
for /f "usebackq delims=" %%M in (`""%GO_EXE%" list -m"`) do set "GAME_MODULE=%%M"
if not defined GAME_MODULE exit /b 1
"%DOTNET_EXE%" "%LUBAN_DLL%" --conf "..\ht_server_game_tables\luban.conf" -t server -c go-json -d json -x "outputCodeDir=common/gamedata/generated" -x "outputDataDir=tables/exported" -x "lubanGoModule=%GAME_MODULE%/common/luban"
if errorlevel 1 exit /b 1
"%GO_EXE%" run ./tools/tablepack %*
exit /b %errorlevel%
