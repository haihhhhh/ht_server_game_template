package config

import (
	"errors"
	"net"
	"strconv"

	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
)

type Config struct {
	base.Base
	Address     string `json:"address"`
	MaxSessions int    `json:"max_sessions"`
}

func Read(path string) (Config, error) {
	gatewayConfig := Config{
		Base:        base.Default(),
		Address:     "127.0.0.1:3250",
		MaxSessions: 128,
	}
	if err := base.Read(path, &gatewayConfig); err != nil {
		return gatewayConfig, err
	}
	gatewayConfig.Environment()
	_, port, err := net.SplitHostPort(gatewayConfig.Address)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || portNumber < 1 || portNumber > 65535 ||
		gatewayConfig.MaxSessions < 1 ||
		gatewayConfig.MaxSessions > 100000 {
		return gatewayConfig, errors.New("config: invalid gateway limits")
	}
	return gatewayConfig, gatewayConfig.Validate()
}
