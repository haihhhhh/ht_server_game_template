// game 是游戏服唯一装配入口；业务细节位于本服务 internal。
package game

import (
	"time"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/host"
	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/runtime"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/config"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/handler"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/storage"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/role"
)

func Run() error {
	gameConfig, err := config.Read(base.Path())
	if err != nil {
		return err
	}
	return runtime.Run("game", gameConfig.Base, func(resources *runtime.Resources) (*app.App, error) {
		idGenerator, buildErr := resources.OpenIDGenerator(gameConfig.Base)
		if buildErr != nil {
			return nil, buildErr
		}
		if openErr := resources.OpenDatabase(gameConfig.Base); openErr != nil {
			return nil, openErr
		}
		if openErr := resources.OpenTables(gameConfig.Base); openErr != nil {
			return nil, openErr
		}
		gameOwnerRegistry, buildErr := resources.Registry(gameConfig.Base, "accounts")
		if buildErr != nil {
			return nil, buildErr
		}
		worldOwnerRegistry, buildErr := resources.Registry(gameConfig.Base, "bosses")
		if buildErr != nil {
			return nil, buildErr
		}

		// 玩家循环由 PlayerManager 托管；角色服务只在请求中调用。
		roleService := &role.Service{
			IDGenerator: idGenerator,
			DB:          resources.DB,
			MaxPerZone:  gameConfig.RolesPerZone,
			Initialize:  storage.RoleInitializer(),
		}
		playerManager := &player.PlayerManager{
			ConfigTables:  resources.Data,
			Clock:         resources.Clock,
			DB:            resources.DB,
			RoleService:   roleService,
			WorldRegistry: worldOwnerRegistry,
			MaxPlayers:    gameConfig.MaxPlayers,
			QueueCapacity: gameConfig.PlayerQueue,
			TickInterval:  time.Duration(gameConfig.TickMS) * time.Millisecond,
			SaveInterval:  time.Duration(gameConfig.SaveSeconds) * time.Second,
		}
		playerSessions := &handler.PlayerSessionRPC{
			PlayerManager: playerManager,
			GameRegistry:  gameOwnerRegistry,
		}
		gameApp, buildErr := runtime.NewApp(
			"game",
			"",
			gameConfig.Base,
			resources,
			host.Entry{
				Name:      "players",
				Module:    playerManager,
				DependsOn: []string{"database", "tables"},
			},
			host.Entry{
				Name:      "player_sessions",
				Module:    &handler.PlayerSessionsLifecycle{Sessions: playerSessions},
				DependsOn: []string{"players"},
			},
		)
		if buildErr != nil {
			return nil, buildErr
		}
		playerManager.App = gameApp
		playerSessions.App = gameApp
		return gameApp, registerRoutes(gameApp, playerSessions, roleService)
	})
}
