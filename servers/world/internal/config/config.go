package config

import (
	"errors"

	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
)

type Config struct {
	base.Base
	MaxScopes     int `json:"max_scopes"`
	QueueCapacity int `json:"queue_capacity"`
	SaveSeconds   int `json:"save_seconds"`
}

func Read(path string) (Config, error) {
	worldConfig := Config{
		Base:          base.Default(),
		MaxScopes:     128,
		QueueCapacity: 64,
		SaveSeconds:   10,
	}
	if err := base.Read(path, &worldConfig); err != nil {
		return worldConfig, err
	}
	worldConfig.Environment()
	if worldConfig.MaxScopes < 1 ||
		worldConfig.MaxScopes > 100000 ||
		worldConfig.QueueCapacity < 1 ||
		worldConfig.QueueCapacity > 4096 ||
		worldConfig.SaveSeconds < 1 ||
		worldConfig.SaveSeconds > 3600 {
		return worldConfig, errors.New("config: invalid world limits")
	}
	return worldConfig, worldConfig.Validate()
}
