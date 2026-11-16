// TODO(业务扩展)【公共服务示例入口】在这里登记个人挑战请求、取出所需组件；网络转发与玩家排队已由上层完成。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package challenge

import (
	"context"

	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
)

func init() {
	modules.RegisterAction(Feature, (*Module).HandleAttackBoss)
}

// TODO(业务扩展)：已在玩家循环内，直接调用自身业务方法。
func (challengeModule *Module) HandleAttackBoss(ctx context.Context, _ *pb.AttackBossReq) (*pb.AttackBossRep, error) {
	return challengeModule.AttackBoss(ctx)
}
