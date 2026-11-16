package account

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
)

const ownershipReleaseInterval = 3 * time.Second

// release 每账号只启动一个退出任务；归属未确认释放期间仍保留退出条目。
func (accountManager *AccountManager) release(id int64, accountEntry *accountSlot) {
	accountManager.mu.Lock()
	if accountEntry.leaving {
		accountManager.mu.Unlock()
		return
	}
	accountEntry.leaving = true
	accountManager.mu.Unlock()
	accountEntry.loop.BeginStop()
	go func() {
		defer func() {
			if recover() != nil {
				slog.Error("账号退出任务发生异常",
					"component", "account", "account_id", id,
					"kind", "panic", "stack", string(debug.Stack()))
			}
		}()
		// 停服可能遇到正在申请归属的首次登录。等结果明确后再决定是否操作 etcd。
		accountEntry.op.Lock()
		releaseOnExit := accountEntry.releaseOnExit
		accountEntry.op.Unlock()
		// 必须先等旧业务实际停止，才能释放归属并允许重新加载。
		_ = accountEntry.loop.Stop(context.Background())
		if !releaseOnExit {
			accountManager.removeSlot(id, accountEntry)
			return
		}
		defer accountManager.releaseOwnership(id, accountEntry)
		if err := accountManager.saveBeforeRelease(id, accountEntry); err != nil {
			slog.Error("账号退出保存未确认，继续释放归属",
				"component", "account", "stage", "leave", "account_id", id)
		}
	}()
}

// 退出只提交剩余变化。即使最后提交失败，释放归属前也等同一账号先前入队的保存。
func (accountManager *AccountManager) saveBeforeRelease(id int64, accountEntry *accountSlot) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("account: final save panic")
			slog.Error("账号最后保存发生异常",
				"component", "account", "stage", "leave", "account_id", id,
				"kind", "panic", "stack", string(debug.Stack()))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = database.WaitForSave(ctx, accountEntry.state.lastSave, err)
	}()
	if accountEntry.loop.Stats().Panics > 0 {
		return errors.New("account: loop panic")
	}
	return accountManager.save(accountEntry.state)
}

// releaseOwnership 每次仅重做原分配的条件删除，已提交的数据库保存不会重复执行。
func (accountManager *AccountManager) releaseOwnership(id int64, accountEntry *accountSlot) {
	for {
		err := accountManager.releaseAssignment(accountEntry.assignment)
		if err == nil {
			break
		}
		slog.Error("账号归属释放未确认，稍后重试",
			"component", "account", "stage", "release", "account_id", id,
			"allocation_id", accountEntry.assignment.AllocationID,
			"retry_seconds", ownershipReleaseInterval.Seconds())
		time.Sleep(ownershipReleaseInterval)
	}
	accountManager.removeSlot(id, accountEntry)
}

func (accountManager *AccountManager) removeSlot(id int64, accountEntry *accountSlot) {
	accountManager.mu.Lock()
	if accountManager.users[id] == accountEntry {
		delete(accountManager.users, id)
	}
	accountManager.mu.Unlock()
	close(accountEntry.closed)
}

// releaseAssignment 只有收到条件删除结果才确认；不匹配说明原归属已不存在或已更换。
func (accountManager *AccountManager) releaseAssignment(assignment placement.Assignment) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("account: ownership release panic")
			slog.Error("账号归属释放发生异常，保留退出标记",
				"component", "account", "stage", "release", "account_id", assignment.Scope,
				"kind", "panic", "stack", string(debug.Stack()))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = accountManager.AccountRegistry.Release(ctx, assignment)
	return err
}
