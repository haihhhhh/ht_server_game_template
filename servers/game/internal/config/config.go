package config

import (
	"errors"

	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
)

type Config struct {
	base.Base
	RolesPerZone int `json:"roles_per_zone"`
	MaxPlayers   int `json:"max_players"`
	PlayerQueue  int `json:"player_queue"`
	TickMS       int `json:"tick_ms"`
	SaveSeconds  int `json:"save_seconds"`
}

func Read(path string) (Config, error) {
	gameConfig := Config{
		Base:         base.Default(),
		RolesPerZone: 1,
		MaxPlayers:   128,
		PlayerQueue:  64,
		TickMS:       1000,
		SaveSeconds:  10,
	}
	if err := base.Read(path, &gameConfig); err != nil {
		return gameConfig, err
	}
	gameConfig.Environment()
	if gameConfig.RolesPerZone < 1 ||
		gameConfig.RolesPerZone > 100 ||
		gameConfig.MaxPlayers < 1 ||
		gameConfig.MaxPlayers > 100000 ||
		gameConfig.PlayerQueue < 1 ||
		gameConfig.PlayerQueue > 4096 ||
		gameConfig.TickMS < 10 ||
		gameConfig.TickMS > 60000 ||
		gameConfig.SaveSeconds < 1 ||
		gameConfig.SaveSeconds > 3600 {
		return gameConfig, errors.New("config: invalid game limits")
	}
	return gameConfig, gameConfig.Validate()
}
