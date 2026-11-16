package player

import (
	"context"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
)

type bindingContextKey struct{}

type playerBinding struct {
	connection clusterlink.Binding
	assignment placement.Assignment
	retired    bool
}

// WithBinding 携带入口已核对的连接和本次分配；排队期间其中任意一项都可能失效。
func WithBinding(ctx context.Context, binding clusterlink.Binding, assignment placement.Assignment) context.Context {
	return context.WithValue(ctx, bindingContextKey{}, playerBinding{connection: binding, assignment: assignment})
}

func bindingFromContext(ctx context.Context) (playerBinding, error) {
	id, err := access.FromContext(ctx)
	binding, ok := ctx.Value(bindingContextKey{}).(playerBinding)
	if err != nil || !ok || binding.connection.Identity != id || binding.connection.Gateway == "" || binding.connection.Token == "" {
		return playerBinding{}, access.ErrUnauthenticated
	}
	return binding, nil
}

func (playerManager *PlayerManager) BindRemote(ctx context.Context, account int64, binding clusterlink.Binding, assignment placement.Assignment) (clusterlink.Binding, error) {
	playerManager.mu.Lock()
	if playerManager.closing {
		playerManager.mu.Unlock()
		return clusterlink.Binding{}, ErrBusy
	}
	playerEntry := playerManager.onlinePlayersByAccount[account]
	if playerEntry == nil {
		previous, err := playerManager.bindLocked(account, binding, assignment)
		playerManager.mu.Unlock()
		return previous, err
	}
	if playerEntry.loading || playerEntry.leaving.Load() {
		playerManager.mu.Unlock()
		return clusterlink.Binding{}, ErrBusy
	}
	playerManager.mu.Unlock()
	// 旧业务先完成，再在同一个循环中切换身份；排在其后的旧请求会在执行前被拒绝。
	out, err := playerEntry.loop.Call(ctx, playerEntry.loopKey, 512, func(context.Context, *Player) (any, error) {
		playerManager.mu.Lock()
		defer playerManager.mu.Unlock()
		if playerManager.closing || playerManager.onlinePlayersByAccount[account] != playerEntry || playerEntry.leaving.Load() {
			return nil, ErrBusy
		}
		return playerManager.bindLocked(account, binding, assignment)
	})
	if err != nil {
		return clusterlink.Binding{}, err
	}
	return out.(clusterlink.Binding), nil
}

// 业务请求校验连接与分配；已授权的本地管理调用只需遵守退出和停服状态。
func (playerManager *PlayerManager) checkBindingLocked(account int64, expected *playerBinding) error {
	current, bound := playerManager.bindingsByAccount[account]
	if expected != nil {
		if !bound || current.connection != expected.connection {
			return access.ErrUnauthenticated
		}
		if current.assignment != expected.assignment {
			return access.ErrForbidden
		}
	}
	if playerManager.closing || current.retired {
		return ErrBusy
	}
	return nil
}

// bindLocked 同时发布连接来源与业务身份，RPC超时不能使两份身份相互矛盾。
func (playerManager *PlayerManager) bindLocked(account int64, binding clusterlink.Binding, assignment placement.Assignment) (clusterlink.Binding, error) {
	current, bound := playerManager.bindingsByAccount[account]
	if current.retired {
		return clusterlink.Binding{}, ErrBusy
	}
	if assignment.Scope != binding.Identity.UserID || binding.Identity.UserID != clusterlink.AccountUID(account) || (bound && current.assignment != assignment) {
		return clusterlink.Binding{}, access.ErrForbidden
	}
	previous := current.connection
	if !binding.Order.Valid() || binding.Order.Before(previous.Order) || (binding.Order == previous.Order && binding != previous) {
		return clusterlink.Binding{}, access.ErrUnauthenticated
	}
	if previous.Gateway == binding.Gateway && previous.Identity.Generation > binding.Identity.Generation {
		return clusterlink.Binding{}, access.ErrUnauthenticated
	}
	playerManager.bindingsByAccount[account] = playerBinding{connection: binding, assignment: assignment}
	return previous, nil
}

// RequestOwner 先核对请求来自当前连接。玩家退出期间，只有当前连接会收到繁忙。
func (playerManager *PlayerManager) RequestOwner(account int64, binding clusterlink.Binding) (placement.Assignment, error) {
	playerManager.mu.RLock()
	defer playerManager.mu.RUnlock()
	current, bound := playerManager.bindingsByAccount[account]
	if !bound || current.connection != binding {
		return placement.Assignment{}, access.ErrUnauthenticated
	}
	if playerManager.closing || current.retired {
		return placement.Assignment{}, ErrBusy
	}
	return current.assignment, nil
}

// RetireBinding 在同一把锁下核对归属、连接并关闭玩家准入，防止中途换绑。
// expected 仅在停服时传 nil，表示关闭该分配当时的连接。没有绑定时返回 false。
func (playerManager *PlayerManager) RetireBinding(account int64, assignment placement.Assignment, expected *clusterlink.Binding) (clusterlink.Binding, bool, error) {
	playerManager.mu.Lock()
	defer playerManager.mu.Unlock()
	current, bound := playerManager.bindingsByAccount[account]
	if expected == nil && !playerManager.closing {
		return clusterlink.Binding{}, bound, access.ErrForbidden
	}
	if !bound {
		return clusterlink.Binding{}, false, nil
	}
	if current.assignment != assignment {
		return clusterlink.Binding{}, true, access.ErrForbidden
	}
	if expected != nil && current.connection != *expected {
		return clusterlink.Binding{}, true, access.ErrUnauthenticated
	}
	current.retired = true
	playerManager.bindingsByAccount[account] = current
	if entry := playerManager.onlinePlayersByAccount[account]; entry != nil {
		entry.retainUntilRelease = true
	}
	return current.connection, true, nil
}

func (playerManager *PlayerManager) HasBinding(account int64) bool {
	playerManager.mu.RLock()
	defer playerManager.mu.RUnlock()
	_, ok := playerManager.bindingsByAccount[account]
	return ok
}

// ForgetRemote 只在原归属已解除后调用，清理原绑定及已经停止的玩家条目。
func (playerManager *PlayerManager) ForgetRemote(account int64, binding clusterlink.Binding, assignment placement.Assignment) {
	playerManager.mu.Lock()
	defer playerManager.mu.Unlock()
	if current, bound := playerManager.bindingsByAccount[account]; bound && current.retired && current.connection == binding && current.assignment == assignment {
		delete(playerManager.bindingsByAccount, account)
		playerEntry := playerManager.onlinePlayersByAccount[account]
		if playerEntry != nil && playerEntry.retainUntilRelease {
			select {
			case <-playerEntry.closed:
				delete(playerManager.onlinePlayersByAccount, account)
			default:
			}
		}
	}
}
