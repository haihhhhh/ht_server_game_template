@echo off
setlocal
cd /d "%~dp0.."
if not defined GO_EXE set "GO_EXE=go"
"%GO_EXE%" run github.com/haihhhhh/ht_server_frame/tools/protocol generate -schema ../ht_server_game_protocol -out common/protocol/generated -catalog ht.game.v1.Messages -rpc -forward Player
exit /b %errorlevel%
