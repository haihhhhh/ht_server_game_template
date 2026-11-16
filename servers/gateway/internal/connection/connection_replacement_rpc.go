package connection

import (
	"context"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/topfreegames/pitaya/v2/component"
)

// ConnectionReplacementRPC 接收其他网关的旧连接替换通知。
type ConnectionReplacementRPC struct {
	component.Base
	App               *app.App
	ConnectionManager *Manager
}

func (connectionsHandler *ConnectionReplacementRPC) Replace(ctx context.Context, q *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.CallRemote(ctx, connectionsHandler.App, "Connections.Replace", false, q, func(ctx context.Context, q *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		id, err := access.FromContext(ctx)
		if err != nil {
			return nil, err
		}
		var old clusterlink.Binding
		if clusterlink.Decode(q, &old) != nil || old.Gateway != connectionsHandler.App.ServerID() || old.Identity.UserID != id.UserID {
			return nil, access.ErrForbidden
		}
		manager := connectionsHandler.ConnectionManager
		if !manager.matchesConnection(old) {
			return &wrappers.BytesValue{}, nil
		}
		err = connectionsHandler.App.CloseReplacedConnection(ctx, old.Identity)
		// Retire 跳过原生断线回调。复用旧名额核对剩余目标，已换绑的后端会忽略旧身份。
		manager.OnDisconnected(old.Identity)
		return &wrappers.BytesValue{}, err
	})
}
