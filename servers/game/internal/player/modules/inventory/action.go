package inventory

import (
	"context"

	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
)

// TODO(业务扩展)：背包协议的入口与道具操作放在同一包。执行时已经进入所属玩家的循环。
func init() { modules.RegisterAction(Feature, (*Module).HandleUseExpItem) }

// TODO(业务扩展)【R09、R10】：这是经验道具请求的业务入口，已在玩家循环内；直接使用自身和已装配依赖，调用规则并组装响应。
func (inventoryModule *Module) HandleUseExpItem(_ context.Context, request *pb.UseExpItemReq) (*pb.UseExpItemRep, error) {
	if err := inventoryModule.UseExpItem(request.ItemId, request.Count); err != nil {
		return nil, err
	}
	// 只返回本次操作相关的当前值，不汇总其他组件或整个背包。
	return &pb.UseExpItemRep{
		ItemId:         request.ItemId,
		RemainingCount: inventoryModule.Count(request.ItemId),
		Experience:     inventoryModule.profile.Experience(),
		Level:          inventoryModule.profile.Level(),
	}, nil
}
