package game

import (
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/handler"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/role"
	"github.com/topfreegames/pitaya/v2/component"
)

func registerRoutes(
	gameApp *app.App,
	playerSessions *handler.PlayerSessionRPC,
	roleService *role.Service,
) error {
	if err := gameApp.RegisterRemote(
		playerSessions,
		component.WithName("Cluster"),
	); err != nil {
		return err
	}
	return gameApp.RegisterRemote(
		&handler.RoleCreationRPC{
			App:         gameApp,
			RoleService: roleService,
		},
		component.WithName("Creation"),
	)
}
