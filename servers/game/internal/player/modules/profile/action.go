package profile

// TODO(业务扩展)：角色资料请求在这里登记；函数执行时已进入玩家循环，可直接读取和修改角色组件。

import (
	"context"

	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
)

func init() {
	modules.RegisterAction(Feature, (*Module).HandleCurrent)
	modules.RegisterAction(Feature, (*Module).HandleSetPreferences)
}
func (profileModule *Module) HandleCurrent(_ context.Context, _ *pb.CurrentReq) (*pb.CurrentRep, error) {
	return &pb.CurrentRep{State: profileModule.currentState()}, nil
}
func (profileModule *Module) HandleSetPreferences(_ context.Context, request *pb.SetPreferencesReq) (*pb.SetPreferencesRep, error) {
	if err := profileModule.SetPreferences(request.Preferences); err != nil {
		return nil, err
	}
	return &pb.SetPreferencesRep{Preferences: profileModule.Data().Preferences}, nil
}
