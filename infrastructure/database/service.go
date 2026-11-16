// 【通常无需修改｜启动、停止】每进程统一管理数据库连接池和多条队列；业务使用 task.go 的入口，不另建连接池。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package database

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/haihhhhh/ht_server_frame/persistence"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
	"gorm.io/gorm"
)

// Service 管理本进程的数据库连接和固定队列，不负责跨节点写入隔离。
type Service struct {
	db       *gorm.DB
	queues   []*persistence.Queue
	once     sync.Once
	done     chan struct{}
	closeErr error
}

// Open 每条队列最多接纳 256 个任务、4 MiB 数据；连接池最多 8 个连接。
func Open(ctx context.Context, dsn string, count int, businessNow func() time.Time) (*Service, error) {
	if count < 1 || count > 8 {
		return nil, errors.New("database: queues must be between 1 and 8")
	}
	if businessNow == nil {
		return nil, errors.New("database: business clock required")
	}
	db, err := openMySQL(ctx, dsn, businessNow)
	if err != nil {
		return nil, err
	}
	if err = migrateTables(ctx, db, model.Models()); err != nil {
		pool, poolErr := db.DB()
		if poolErr == nil {
			_ = pool.Close()
		}
		return nil, err
	}
	databaseService := &Service{
		db:   db,
		done: make(chan struct{}),
	}
	for i := 0; i < count; i++ {
		queue, queueErr := persistence.NewQueue(persistence.Options{
			Capacity: 256,
			MaxBytes: 4 << 20,
			Timeout:  3 * time.Second,
		})
		if queueErr != nil {
			_ = databaseService.Close()
			return nil, queueErr
		}
		databaseService.queues = append(databaseService.queues, queue)
	}
	return databaseService, nil
}

func (databaseService *Service) Stats() persistence.Stats {
	var total persistence.Stats
	for _, q := range databaseService.queues {
		v := q.Stats()
		total.Pending += v.Pending
		total.Bytes += v.Bytes
		total.Completed += v.Completed
		total.Failed += v.Failed
		total.Rejected += v.Rejected
	}
	return total
}

func (databaseService *Service) Init(context.Context) error {
	return nil
}

// Close 先关闭所有队列准入，等待真实执行结束后才关闭连接池；超时仅结束调用方等待。
func (databaseService *Service) Close() error {
	databaseService.once.Do(func() {
		for _, q := range databaseService.queues {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_ = q.Close(ctx)
		}
		go func() {
			defer close(databaseService.done)
			defer func() {
				if recover() != nil {
					logging.Panic("database_close")
					databaseService.closeErr = errors.New("database: close panic")
				}
			}()
			for _, q := range databaseService.queues {
				_ = q.Close(context.Background())
			}
			pool, err := databaseService.db.DB()
			if err == nil {
				err = pool.Close()
			}
			databaseService.closeErr = err
		}()
	})
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-databaseService.done:
		return databaseService.closeErr
	case <-timer.C:
		return errors.New("database: close timed out; draining continues")
	}
}
