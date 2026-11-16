package boss

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/execution"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
	"github.com/haihhhhh/ht_server_game_template/servers/world/internal/boss/storage"
)

// 同一范围共享一次准备结果；若需要撤销登记，done 在原候选清理结束后才关闭。
type bossClaim struct {
	done  chan struct{}
	owner placement.Assignment
	err   error
}

func (claim *bossClaim) wait(ctx context.Context) (placement.Assignment, error) {
	select {
	case <-claim.done:
		return claim.owner, claim.err
	case <-ctx.Done():
		return placement.Assignment{}, ctx.Err()
	}
}

func (bossService *Service) claim(ctx context.Context, candidate placement.Assignment) (placement.Assignment, error) {
	bossService.mu.Lock()
	if bossService.closing {
		bossService.mu.Unlock()
		return placement.Assignment{}, execution.ErrStopped
	}
	if state := bossService.states[candidate.Scope]; state != nil {
		bossService.mu.Unlock()
		return state.assignment, nil
	}
	if pending := bossService.preparing[candidate.Scope]; pending != nil {
		bossService.mu.Unlock()
		return pending.wait(ctx)
	}
	if len(bossService.states)+len(bossService.preparing) >= bossService.MaxScopes {
		bossService.mu.Unlock()
		return placement.Assignment{}, execution.ErrOverloaded
	}
	pending := &bossClaim{done: make(chan struct{})}
	bossService.preparing[candidate.Scope] = pending
	bossService.mu.Unlock()
	return bossService.prepare(ctx, candidate, pending)
}

func (bossService *Service) prepare(ctx context.Context, candidate placement.Assignment, pending *bossClaim) (owner placement.Assignment, err error) {
	var state *bossState
	// Claim 超时时 etcd 可能已经登记。失败只撤销本次候选，不删除其他 world 的归属。
	defer func() {
		failure := recover()
		if failure != nil {
			err = errors.New("world: boss preparation panicked")
		}
		bossService.mu.Lock()
		if bossService.closing && state != nil {
			// 准备结束时已开始停服，尚未开放攻击；撤销候选即可，无需保存未修改的血量。
			owner, err = placement.Assignment{}, execution.ErrStopped
		}
		pending.owner, pending.err = owner, err
		if err != nil {
			go bossService.releaseFailedClaim(candidate, pending)
		} else {
			if state != nil {
				bossService.states[candidate.Scope] = state
			}
			delete(bossService.preparing, candidate.Scope)
			close(pending.done)
		}
		bossService.mu.Unlock()
		if failure != nil {
			panic(failure)
		}
	}()

	owner, err = bossService.WorldRegistry.Claim(ctx, candidate)
	if err != nil {
		return placement.Assignment{}, err
	}
	if owner.ServerID != candidate.ServerID || owner.AllocationID != candidate.AllocationID {
		return owner, nil
	}
	// 取得归属后才读数据库，避免读取上一个正常退出节点最后一次保存之前的血量。
	value, err := storage.Load(ctx, bossService.DB, candidate.Scope)
	if err != nil {
		return placement.Assignment{}, err
	}
	state = &bossState{data: value, assignment: owner}
	return owner, nil
}

func (bossService *Service) releaseFailedClaim(candidate placement.Assignment, pending *bossClaim) {
	for {
		err := bossService.releaseCandidate(candidate)
		if err == nil {
			bossService.mu.Lock()
			delete(bossService.preparing, candidate.Scope)
			close(pending.done)
			bossService.mu.Unlock()
			return
		}
		slog.Error("Boss 准备失败后的归属清理未完成，稍后重试", "scope", candidate.Scope, "error", err)
		time.Sleep(3 * time.Second)
	}
}

func (bossService *Service) releaseCandidate(candidate placement.Assignment) (err error) {
	defer func() {
		if recover() != nil {
			logging.Panic("boss_claim_cleanup")
			err = errors.New("world: boss claim cleanup panicked")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = bossService.WorldRegistry.Release(ctx, candidate)
	return err
}

// game 可能在 Boss 加载结束前发来攻击。等待本机这次加载完成，再核对请求的分配编号。
func (bossService *Service) waitReady(ctx context.Context, scope, allocationID string) error {
	bossService.mu.Lock()
	state, pending := bossService.states[scope], bossService.preparing[scope]
	bossService.mu.Unlock()
	if pending != nil {
		if _, err := pending.wait(ctx); err != nil {
			return err
		}
		bossService.mu.Lock()
		state = bossService.states[scope]
		bossService.mu.Unlock()
	}
	if state != nil && state.assignment.AllocationID == allocationID {
		return nil
	}
	return access.ErrForbidden
}
