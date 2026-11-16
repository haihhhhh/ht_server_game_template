package gateway

import (
	"github.com/haihhhhh/ht_server_frame/app"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/gateway/internal/connection"
	"github.com/haihhhhh/ht_server_game_template/servers/gateway/internal/handler"
	"github.com/topfreegames/pitaya/v2/component"
)

func registerRoutes(gatewayApp *app.App, connectionManager *connection.Manager) error {
	// TODO(业务扩展)：登录、选角等连接协调接口在这里注册；普通 Player 协议由生成的转发组件统一接收。
	if err := gatewayApp.Register(
		&handler.Account{
			App:               gatewayApp,
			ConnectionManager: connectionManager,
		},
	); err != nil {
		return err
	}
	if err := gatewayApp.Register(
		&handler.Roles{
			App:               gatewayApp,
			ConnectionManager: connectionManager,
		},
	); err != nil {
		return err
	}
	if err := gatewayApp.Register(
		&pb.PlayerForwarder{
			Application:    gatewayApp,
			ForwardRequest: connectionManager.ForwardPlayer,
		},
		component.WithName("Player"),
	); err != nil {
		return err
	}
	if err := gatewayApp.RegisterRemote(
		&connection.ConnectionReplacementRPC{
			App:               gatewayApp,
			ConnectionManager: connectionManager,
		},
		component.WithName("Connections"),
	); err != nil {
		return err
	}
	return gatewayApp.RegisterRemote(
		&connection.ConnectionPushRPC{
			App:               gatewayApp,
			ConnectionManager: connectionManager,
		},
		component.WithName("ConnectionPush"),
	)
}
