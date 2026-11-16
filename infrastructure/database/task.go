// 【通常无需修改｜S03】按账号等固定键选择 DB 队列；Submit 返回执行结果通道，Query 等待查询结果。回调不要再次等待本队列。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package database

import (
	"context"
	"errors"
	"hash/fnv"
	"strconv"

	"github.com/haihhhhh/ht_server_frame/observability"
	"gorm.io/gorm"
)

// AccountKey 将角色创建、角色加载和模块保存归入同一个账号通道。
// 当前骨架同账号只在线一个角色；账号内多个角色也保守顺序执行。
func AccountKey(accountID int64) string {
	return "account:" + strconv.FormatInt(accountID, 10)
}

// WaitForSave 供退出流程调用一次。同键任务按序执行，最后一笔结束时，前面的任务也已结束。
// 即使最后的变更没能入队，也要等先前已提交的保存，并保留本次提交错误。
func WaitForSave(ctx context.Context, lastSave <-chan error, submitErr error) error {
	if lastSave == nil {
		return submitErr
	}
	select {
	case writeErr := <-lastSave:
		return errors.Join(submitErr, writeErr)
	case <-ctx.Done():
		return errors.Join(submitErr, ctx.Err())
	}
}

// Submit 只接收独立的数据副本。回调不得再次等待本服务，否则可能等待自身队列。
// 后台保存不继承请求取消，执行时由队列设置独立超时。
func (databaseService *Service) Submit(ctx context.Context, key string, size int64, fn func(*gorm.DB) error) (<-chan error, error) {
	if key == "" || fn == nil {
		return nil, errors.New("database: key and operation required")
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	q := databaseService.queues[h.Sum64()%uint64(len(databaseService.queues))]
	return q.SubmitContext(ctx, func(task context.Context) error {
		err := fn(databaseService.db.WithContext(task))
		if err != nil {
			observability.Logger(task).ErrorContext(task, "数据库任务失败", "component", "database", "stage", "execute", "kind", "database_error")
		}
		return err
	}, size)
}

// Query 的结果通过独立通道交付，调用方超时后不再读取执行线程的可变变量。
// 排队时已取消的查询跳过；运行中的查询同时受请求取消与队列执行超时约束。
func Query[T any](ctx context.Context, databaseService *Service, key string, size int64, fn func(*gorm.DB) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	values := make(chan T, 1)
	done, err := databaseService.Submit(ctx, key, size, func(db *gorm.DB) error {
		task, cancel := context.WithCancel(db.Statement.Context)
		defer cancel()
		unlink := context.AfterFunc(ctx, cancel)
		defer unlink()
		if err := ctx.Err(); err != nil {
			return err
		}
		value, err := fn(db.WithContext(task))
		if err == nil {
			values <- value
		}
		return err
	})
	if err != nil {
		return zero, err
	}
	select {
	case executionErr := <-done:
		if executionErr != nil {
			return zero, executionErr
		}
		return <-values, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}
