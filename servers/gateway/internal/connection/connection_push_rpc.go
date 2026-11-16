package connection

import (
	"context"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/topfreegames/pitaya/v2/component"
)

// ConnectionPushRPC 只注册内部 Remote；服务认证由 NATS 提供，目标按已有连接绑定核对。
type ConnectionPushRPC struct {
	component.Base
	App               *app.App
	ConnectionManager *Manager
}

func (pushHandler *ConnectionPushRPC) Push(ctx context.Context, packet *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.CallRemote(ctx, pushHandler.App, "ConnectionPush.Push", false, packet, func(ctx context.Context, packet *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		id, err := access.FromContext(ctx)
		if err != nil {
			return nil, err
		}
		var push clusterlink.ConnectionPush
		if clusterlink.Decode(packet, &push) != nil || push.Binding.Gateway != pushHandler.App.ServerID() || push.Binding.Identity != id {
			return nil, access.ErrForbidden
		}
		if !pushHandler.ConnectionManager.matchesConnection(push.Binding) {
			return nil, access.ErrUnauthenticated
		}
		message, err := push.Message()
		if err != nil {
			return nil, err
		}
		if err = pushHandler.App.PushToConnection(ctx, id, message); err != nil {
			return nil, err
		}
		return &wrappers.BytesValue{}, nil
	})
}
