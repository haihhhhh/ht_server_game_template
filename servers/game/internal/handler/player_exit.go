package handler

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player"
)

// beginExit 在账号操作锁内确认要退出的连接。返回后调用方在锁外等待同一个后台任务。
func (playerSessionRPC *PlayerSessionRPC) beginExit(ctx context.Context, accountID int64, command clusterlink.Command, binding clusterlink.Binding) (*owned, error) {
	entry, _, err := playerSessionRPC.lockEntry(accountID, command.Action)
	if err != nil || entry == nil {
		return nil, err
	}
	defer entry.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if entry.retired {
		if binding == entry.exitBinding && command.Allocation == entry.assignment.AllocationID {
			return entry, nil
		}
		return nil, player.ErrBusy
	}
	assigned, err := playerSessionRPC.checkOwner(ctx, binding.Identity.UserID, command.Allocation, entry.assignment)
	if err != nil {
		return nil, err
	}
	_, bound, err := playerSessionRPC.PlayerManager.RetireBinding(accountID, assigned, &binding)
	if err != nil {
		if command.Action == clusterlink.Disconnect && err == access.ErrUnauthenticated {
			return nil, nil // 迟到的旧连接通知不能让当前连接下线。
		}
		return nil, err
	}
	// 首次关联可能只登记了归属；断线仍要回收，主动退出则要求已有绑定。
	if !bound && command.Action != clusterlink.Disconnect {
		return nil, access.ErrUnauthenticated
	}
	playerSessionRPC.startExit(ctx, accountID, entry, assigned, binding, bound)
	return entry, nil
}

// startExit 在会话锁内启动一次。已有绑定须先关闭准入，RPC 在锁外等待结果。
func (playerSessionRPC *PlayerSessionRPC) startExit(ctx context.Context, account int64, entry *owned, assignment placement.Assignment, binding clusterlink.Binding, hasBinding bool) {
	entry.retired = true
	entry.assignment = assignment
	entry.exitBinding = binding
	entry.exitDone = make(chan struct{})
	background := context.WithoutCancel(ctx)
	go func() {
		defer close(entry.exitDone)
		defer func() {
			if recovered := recover(); recovered != nil {
				entry.exitErr = fmt.Errorf("Player: exit panic: %v", recovered)
				slog.Error("玩家退出异常，保留退出条目", "account_id", account, "stack", string(debug.Stack()))
			}
		}()
		entry.saved = true
		if hasBinding {
			entry.saved, entry.exitErr = playerSessionRPC.PlayerManager.Leave(background, account, binding)
			if entry.exitErr != nil {
				return
			}
		}
		// 这里只重试原归属的条件删除，不重新调用下线钩子或重复保存。
		for {
			if err := playerSessionRPC.releaseAssignment(assignment); err == nil {
				break
			} else {
				slog.Error("玩家归属释放失败，后台稍后重试", "account_id", account, "allocation_id", assignment.AllocationID, "error", err, "retry_seconds", 3)
			}
			time.Sleep(3 * time.Second)
		}
		if hasBinding {
			playerSessionRPC.PlayerManager.ForgetRemote(account, binding, assignment)
		}
		playerSessionRPC.mu.Lock()
		defer playerSessionRPC.mu.Unlock()
		if playerSessionRPC.users[account] == entry {
			delete(playerSessionRPC.users, account)
		}
	}()
}

func (playerSessionRPC *PlayerSessionRPC) waitExit(ctx context.Context, entry *owned) (*wrappers.BytesValue, error) {
	select {
	case <-entry.exitDone:
		if entry.exitErr != nil {
			return nil, entry.exitErr
		}
		return clusterlink.EncodeReply(&pb.LeaveRep{SaveConfirmed: entry.saved})
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (playerSessionRPC *PlayerSessionRPC) releaseAssignment(assignment placement.Assignment) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("Player: assignment release panic: %v", recovered)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// false 且无错误表示原记录已经不存在或已被替换，同样无需继续删除。
	_, err = playerSessionRPC.GameRegistry.Release(ctx, assignment)
	return err
}
