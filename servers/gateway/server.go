// gateway 协调登录和连接，账号验证交给 account，网络收发由 Pitaya 管理。
package gateway

import (
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/host"
	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/runtime"
	"github.com/haihhhhh/ht_server_game_template/servers/gateway/internal/config"
	"github.com/haihhhhh/ht_server_game_template/servers/gateway/internal/connection"
)

func Run() error {
	gatewayConfig, err := config.Read(base.Path())
	if err != nil {
		return err
	}
	return runtime.Run("gateway", gatewayConfig.Base, func(resources *runtime.Resources) (*app.App, error) {
		gameOwnerRegistry, buildErr := resources.Registry(gatewayConfig.Base, "accounts")
		if buildErr != nil {
			return nil, buildErr
		}

		accountOwnerRegistry, buildErr := resources.Registry(gatewayConfig.Base, "account-owners")
		if buildErr != nil {
			return nil, buildErr
		}
		// ConnectionManager 托管连接绑定与断线通知。
		connectionManager := &connection.Manager{
			GameRegistry:    gameOwnerRegistry,
			AccountRegistry: accountOwnerRegistry,
			MaxSessions:     gatewayConfig.MaxSessions,
		}
		gatewayApp, buildErr := runtime.NewApp(
			"gateway",
			gatewayConfig.Address,
			gatewayConfig.Base,
			resources,
			host.Entry{
				Name:   "connections",
				Module: connectionManager,
			},
		)
		if buildErr != nil {
			return nil, buildErr
		}
		connectionManager.App = gatewayApp
		if buildErr = configureAuthentication(gatewayApp, connectionManager, gatewayConfig.MaxSessions); buildErr != nil {
			return nil, buildErr
		}
		return gatewayApp, registerRoutes(gatewayApp, connectionManager)
	})
}
