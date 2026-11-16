#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
: "${GO_EXE:=go}"
: "${DOTNET_EXE:=dotnet}"
: "${LUBAN_DLL:=../.tools/luban-5.1.0/Luban/Luban.dll}"
export GOWORK=off
game_module=$("$GO_EXE" list -m)
"$DOTNET_EXE" "$LUBAN_DLL" --conf ../ht_server_game_tables/luban.conf -t server -c go-json -d json -x outputCodeDir=common/gamedata/generated -x outputDataDir=tables/exported -x "lubanGoModule=$game_module/common/luban"
"$GO_EXE" run ./tools/tablepack "$@"
