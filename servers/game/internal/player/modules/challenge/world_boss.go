// TODO(业务扩展)【公共服务示例】个人校验后调用 world，再修改个人次数；这里不是经验道具主线，RPC 超时也可能已经扣除 Boss 血量。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package challenge

import (
	"context"
	"errors"

	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/common/worldlink"
)

// AttackBoss 在玩家循环内执行。首次先向候选 world 认领 Boss，再向实际负责人发一次攻击。
func (challengeModule *Module) AttackBoss(ctx context.Context) (*pb.AttackBossRep, error) {
	if challengeModule.Remaining() <= 0 {
		return nil, errcode.ErrNoChallenges
	}
	scope := worldlink.Scope(challengeModule.zone)
	assigned, found, err := challengeModule.world.Registry.Lookup(ctx, scope)
	if err != nil {
		return nil, err
	}
	if !found {
		candidate, selectErr := clusterlink.SelectCandidate(challengeModule.world.App, "world", scope)
		if errors.Is(selectErr, clusterlink.ErrNoCandidate) {
			return nil, errcode.ErrWorldUnavailable
		}
		if selectErr != nil {
			return nil, selectErr
		}
		// 候选 world 只认领，不扣血；并发竞争时返回已经胜出的节点。
		assigned, err = worldlink.Claim(ctx, challengeModule.world.App, candidate)
		if err != nil {
			return nil, err
		}
	}
	// 伤害属于项目计算，这里固定十点展示调用流程，不接受客户端传入。
	result, err := worldlink.CallAttack(ctx, challengeModule.world.App, assigned.ServerID, worldlink.Attack{
		Scope:      scope,
		Allocation: assigned.AllocationID,
		Damage:     10,
	})
	if err != nil {
		return nil, err
	}
	challengeModule.Consume()
	return &pb.AttackBossRep{
		BossHp:         result.HP,
		ChallengesLeft: challengeModule.Remaining(),
	}, nil
}
