package config

import (
	"errors"

	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/directory"
)

type Config struct {
	base.Base
	MaxAccounts   int              `json:"max_accounts"`
	QueueCapacity int              `json:"account_queue"`
	SaveSeconds   int              `json:"save_seconds"`
	Zones         []directory.Zone `json:"zones"`
}

func Read(path string) (Config, error) {
	accountConfig := Config{
		Base:          base.Default(),
		MaxAccounts:   128,
		QueueCapacity: 64,
		SaveSeconds:   10,
	}
	accountConfig.Zones = []directory.Zone{{
		ID:   "default",
		Name: "默认区",
		Open: true,
	}}
	if err := base.Read(path, &accountConfig); err != nil {
		return accountConfig, err
	}
	accountConfig.Environment()
	if err := directory.Validate(accountConfig.Zones); err != nil {
		return accountConfig, err
	}
	if accountConfig.MaxAccounts < 1 ||
		accountConfig.MaxAccounts > 100000 ||
		accountConfig.QueueCapacity < 1 ||
		accountConfig.QueueCapacity > 4096 ||
		accountConfig.SaveSeconds < 1 ||
		accountConfig.SaveSeconds > 3600 {
		return accountConfig, errors.New("account: invalid limits")
	}
	return accountConfig, accountConfig.Validate()
}
