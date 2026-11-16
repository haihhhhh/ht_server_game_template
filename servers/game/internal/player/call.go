// 【通常无需修改｜R08】找到当前在线玩家，排入其循环，执行前再次核对连接和就绪状态；业务只能在回调内访问玩家。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package player

import (
	"context"
	"errors"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/execution"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
)

// Call 只在所属玩家循环中交付对象；回调不得保存对象引用或等待自己的循环。
// 返回值必须是独立结果，不能把可变业务字段交给循环外的调用者。
func (playerManager *PlayerManager) Call(ctx context.Context, account int64, size int64, run func(context.Context, *Player) (any, error)) (any, error) {
	binding, err := bindingFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if binding.connection.Identity.UserID != clusterlink.AccountUID(account) {
		return nil, access.ErrForbidden
	}
	return playerManager.call(ctx, account, &binding, size, run)
}

// CallTrusted 供已授权的本服务管理入口调用；只处理本节点在线对象，不加载离线玩家或绕过归属选择。
func (playerManager *PlayerManager) CallTrusted(ctx context.Context, account int64, size int64, run func(context.Context, *Player) (any, error)) (any, error) {
	return playerManager.call(ctx, account, nil, size, run)
}

// binding 仅在本服务已授权的 CallTrusted 入口为空。
func (playerManager *PlayerManager) entryForCall(account int64, binding *playerBinding) (*onlinePlayerEntry, error) {
	playerManager.mu.RLock()
	defer playerManager.mu.RUnlock()
	if err := playerManager.checkBindingLocked(account, binding); err != nil {
		return nil, err
	}
	entry := playerManager.onlinePlayersByAccount[account]
	if entry == nil || entry.loading || entry.leaving.Load() {
		return nil, ErrBusy
	}
	return entry, nil
}

func (playerManager *PlayerManager) call(ctx context.Context, account int64, binding *playerBinding, size int64, run func(context.Context, *Player) (any, error)) (any, error) {
	if run == nil {
		return nil, execution.ErrInvalid
	}
	playerEntry, err := playerManager.entryForCall(account, binding)
	if err != nil {
		return nil, err
	}
	out, err := playerEntry.loop.Call(ctx, playerEntry.loopKey, size, func(task context.Context, onlinePlayer *Player) (any, error) {
		if checkErr := playerManager.checkQueued(account, binding, playerEntry); checkErr != nil {
			return nil, checkErr
		}
		if !onlinePlayer.ready {
			return nil, ErrBusy
		}
		if err := task.Err(); err != nil {
			return nil, err
		}
		return run(task, onlinePlayer)
	})
	if errors.Is(err, execution.ErrFaulted) {
		playerManager.release(account, playerEntry)
	}
	return out, err
}
