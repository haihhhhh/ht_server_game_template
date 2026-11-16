package main

import (
	"os"

	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
	"github.com/haihhhhh/ht_server_game_template/servers/game"
)

func main() {
	os.Exit(logging.Run("game", game.Run))
}
