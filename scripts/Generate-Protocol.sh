#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
"${GO_EXE:-go}" run github.com/haihhhhh/ht_server_frame/tools/protocol generate -schema ../ht_server_game_protocol -out common/protocol/generated -catalog ht.game.v1.Messages -rpc -forward Player
