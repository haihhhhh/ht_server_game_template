package boss

import (
	"context"
	"strings"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/execution"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	"github.com/haihhhhh/ht_server_game_template/common/worldlink"
	"github.com/topfreegames/pitaya/v2/component"
)

// 服务认证交给 NATS；此入口仍检查参数和 Boss 归属，不把内部调用当作玩家登录证明。
// BossRPC 接收 game 发来的公共 Boss 请求。
type BossRPC struct {
	component.Base
	App         *app.App
	BossService *Service
}

func validScope(scope string) bool {
	return strings.HasPrefix(scope, "boss:") && len(scope) > len("boss:") && len(scope) <= 256
}

// Claim 预留本地容量并准备 Boss，只返回归属。攻击始终通过另一次普通 RPC 执行。
func (bossHandler *BossRPC) Claim(ctx context.Context, request *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.Call(ctx, bossHandler.App, "Boss.Claim", false, request, func(ctx context.Context, request *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		var candidate placement.Assignment
		if clusterlink.Decode(request, &candidate) != nil || !validScope(candidate.Scope) || candidate.AllocationID == "" || len(candidate.AllocationID) > 128 || candidate.ServerID != bossHandler.App.ServerID() {
			return nil, execution.ErrInvalid
		}
		owner, err := bossHandler.BossService.claim(ctx, candidate)
		if err != nil {
			return nil, err
		}
		return clusterlink.Encode(owner)
	})
}

func (bossHandler *BossRPC) Attack(ctx context.Context, request *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.Call(ctx, bossHandler.App, "Boss.Attack", false, request, func(ctx context.Context, request *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		var q worldlink.Attack
		if clusterlink.Decode(request, &q) != nil {
			return nil, access.ErrForbidden
		}
		if !validScope(q.Scope) || q.Damage < 1 || q.Damage > 100 || q.Allocation == "" {
			return nil, execution.ErrInvalid
		}
		if err := bossHandler.BossService.waitReady(ctx, q.Scope, q.Allocation); err != nil {
			return nil, err
		}
		out, err := bossHandler.BossService.loop.Call(ctx, q.Scope, 512, func(task context.Context, state *bossState) (any, error) {
			if state.assignment.AllocationID != q.Allocation {
				return nil, access.ErrForbidden
			}
			if taskErr := task.Err(); taskErr != nil {
				return nil, taskErr
			}
			// TODO(业务扩展)：Boss 伤害、死亡等规则在本 Boss 的循环内执行，修改后标记 dirty。
			if state.data.HP <= 0 {
				return nil, errcode.ErrBossDefeated
			}
			state.data.HP = max(0, state.data.HP-q.Damage)
			state.dirty = true
			return worldlink.Result{HP: state.data.HP}, nil
		})
		if err != nil {
			return nil, errcode.Public(err)
		}
		return clusterlink.Encode(out.(worldlink.Result))
	})
}
