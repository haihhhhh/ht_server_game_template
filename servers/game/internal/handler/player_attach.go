package handler

import (
	"context"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player"
)

// 首次关联先占本机名额；全局索引锁只保护条目，登记归属时只持有本账号的锁。
func (sessions *PlayerSessionRPC) lockEntry(account int64, action clusterlink.Action) (*owned, bool, error) {
	sessions.mu.Lock()
	if sessions.closing {
		sessions.mu.Unlock()
		return nil, false, player.ErrBusy
	}
	entry := sessions.users[account]
	if entry == nil {
		if action == clusterlink.Disconnect {
			sessions.mu.Unlock()
			return nil, false, nil
		}
		if action != clusterlink.Attach || len(sessions.users) >= sessions.PlayerManager.MaxPlayers {
			sessions.mu.Unlock()
			return nil, false, player.ErrBusy
		}
		if sessions.users == nil {
			sessions.users = make(map[int64]*owned)
		}
		entry = &owned{}
		entry.mu.Lock()
		sessions.users[account] = entry
		sessions.mu.Unlock()
		return entry, true, nil
	}
	sessions.mu.Unlock()
	if action == clusterlink.Disconnect {
		entry.mu.Lock()
	} else if !entry.mu.TryLock() {
		return nil, false, player.ErrBusy
	}
	return entry, false, nil
}

func (sessions *PlayerSessionRPC) discardReservation(account int64, entry *owned) {
	entry.retired = true
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.users[account] == entry {
		delete(sessions.users, account)
	}
}

func (sessions *PlayerSessionRPC) attach(ctx context.Context, account int64, command clusterlink.Command, binding clusterlink.Binding) (*wrappers.BytesValue, error) {
	entry, fresh, err := sessions.lockEntry(account, clusterlink.Attach)
	if err != nil {
		return nil, err
	}
	defer entry.mu.Unlock()
	if fresh {
		defer func() {
			if entry.assignment.AllocationID == "" {
				sessions.discardReservation(account, entry)
			}
		}()
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if entry.retired {
		return nil, player.ErrBusy
	}
	target := placement.Assignment{Scope: binding.Identity.UserID, ServerID: sessions.App.ServerID(), AllocationID: command.Allocation}
	var assigned placement.Assignment
	if fresh && command.Claim {
		// 先记住本次编号，即使登记回包丢失，也能仅撤销自己的候选。
		assigned, err = sessions.GameRegistry.Claim(ctx, target)
		if err != nil {
			sessions.startExit(ctx, account, entry, target, binding, false)
			return nil, err
		}
	} else {
		var found bool
		assigned, found, err = sessions.GameRegistry.Lookup(ctx, target.Scope)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, access.ErrForbidden
		}
	}
	if assigned.ServerID != target.ServerID || assigned.AllocationID != target.AllocationID {
		if command.Claim {
			return clusterlink.Encode(clusterlink.Reply{Redirect: &assigned})
		}
		return nil, access.ErrForbidden
	}
	if !fresh && entry.assignment != assigned {
		return nil, access.ErrForbidden
	}
	entry.assignment = assigned
	previous, err := sessions.PlayerManager.BindRemote(ctx, account, binding, assigned)
	if err != nil {
		if fresh && !sessions.PlayerManager.HasBinding(account) {
			sessions.startExit(ctx, account, entry, assigned, binding, false)
		}
		return nil, err
	}
	return clusterlink.Encode(clusterlink.Reply{Previous: previous, Assignment: &assigned})
}
