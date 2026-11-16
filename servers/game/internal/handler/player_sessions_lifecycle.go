package handler

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PlayerSessionsLifecycle 将同一个 RPC 会话管理对象接入 host，避免与 Pitaya 的 Init() 签名冲突。
// 必须先于 PlayerManager 和数据库关闭；这里只协调现有退出任务，不另写保存流程。
type PlayerSessionsLifecycle struct {
	Sessions *PlayerSessionRPC
}

func (*PlayerSessionsLifecycle) Init(context.Context) error { return nil }

func (lifecycle *PlayerSessionsLifecycle) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return lifecycle.Sessions.closeSessions(ctx)
}

func (playerSessionRPC *PlayerSessionRPC) closeSessions(ctx context.Context) error {
	playerSessionRPC.PlayerManager.BeginStop()
	playerSessionRPC.mu.Lock()
	playerSessionRPC.closing = true
	entries := make(map[int64]*owned, len(playerSessionRPC.users))
	for account, entry := range playerSessionRPC.users {
		entries[account] = entry
	}
	playerSessionRPC.mu.Unlock()

	var failures []error
	var waiting []*owned
	for account, entry := range entries {
		// host 已停止准入并排空 RPC，不持有管理器锁等待玩家退出。
		entry.mu.Lock()
		if entry.assignment.AllocationID == "" && entry.exitDone == nil {
			// 预留位置尚未认领任何归属；不能用停服时查到的记录代替本次分配。
			playerSessionRPC.discardReservation(account, entry)
			entry.mu.Unlock()
			continue
		}
		if !entry.retired {
			binding, bound, err := playerSessionRPC.PlayerManager.RetireBinding(account, entry.assignment, nil)
			if err != nil {
				entry.mu.Unlock()
				failures = append(failures, fmt.Errorf("账号 %d 的退出归属不匹配: %w", account, err))
				continue
			}
			playerSessionRPC.startExit(context.Background(), account, entry, entry.assignment, binding, bound)
		}
		waiting = append(waiting, entry)
		entry.mu.Unlock()
	}
	for _, entry := range waiting {
		select {
		case <-entry.exitDone:
			if entry.exitErr != nil {
				failures = append(failures, entry.exitErr)
			}
		case <-ctx.Done():
			return errors.Join(append(failures, fmt.Errorf("玩家归属收尾未完成: %w", ctx.Err()))...)
		}
	}
	return errors.Join(failures...)
}
