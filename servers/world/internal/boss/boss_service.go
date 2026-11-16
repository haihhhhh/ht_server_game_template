package boss

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/execution"
	"github.com/haihhhhh/ht_server_frame/placement"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
	"github.com/haihhhhh/ht_server_game_template/servers/world/internal/boss/storage"
)

type bossState struct {
	data       model.WorldBoss
	assignment placement.Assignment
	dirty      bool
	lastSave   <-chan error
}

// Service 是项目公共业务；每个 Boss 范围一个循环，公共状态不随玩家离线释放。
type Service struct {
	App           *app.App
	DB            *database.Service
	WorldRegistry *placement.Registry
	MaxScopes     int
	QueueCapacity int
	SaveInterval  time.Duration
	loop          *execution.Executor[*bossState]
	mu            sync.Mutex
	states        map[string]*bossState
	preparing     map[string]*bossClaim
	closing       bool
}

func (bossService *Service) Init(context.Context) error {
	if bossService.MaxScopes < 1 || bossService.QueueCapacity < 1 || bossService.SaveInterval <= 0 {
		return errors.New("world: invalid loop limits")
	}
	bossService.states = make(map[string]*bossState)
	bossService.preparing = make(map[string]*bossClaim)
	var initErr error
	bossService.loop, initErr = execution.New(execution.Options[*bossState]{
		Name:          "world-boss",
		MaxDomains:    bossService.MaxScopes,
		QueueCapacity: bossService.QueueCapacity,
		MaxQueueBytes: 1 << 20,
		InitTimeout:   3 * time.Second,
		Init: func(_ context.Context, scope string) (*bossState, error) {
			bossService.mu.Lock()
			defer bossService.mu.Unlock()
			state := bossService.states[scope]
			if state == nil {
				return nil, access.ErrForbidden
			}
			return state, nil
		},
		UpdateInterval: bossService.SaveInterval,
		Update: func(_ context.Context, state *bossState) error {
			return bossService.save(state)
		},
	})
	return initErr
}
func (bossService *Service) save(state *bossState) error {
	if !state.dirty {
		return nil
	}
	done, err := storage.Save(bossService.DB, state.data)
	if err == nil {
		state.dirty = false
		state.lastSave = done
	}
	return err
}

// Close 等所有循环停止后逐个收尾，防止把异常中断的 Boss 状态写回数据库。
func (bossService *Service) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	bossService.mu.Lock()
	bossService.closing = true
	preparing := maps.Clone(bossService.preparing)
	bossService.mu.Unlock()
	if err := bossService.loop.Stop(ctx); err != nil {
		return err
	}
	var failures []error
	for scope, state := range bossService.states {
		if err := bossService.closeBoss(ctx, state); err != nil {
			slog.Error("Boss 停服收尾失败", "component", "world-boss", "scope", scope, "error", err)
			failures = append(failures, fmt.Errorf("world: boss %q: %w", scope, err))
		}
	}
	// 未完成的首次认领单独收尾，不因它的撤销较慢而跳过其他 Boss 的最后保存。
	for scope, claim := range preparing {
		select {
		case <-claim.done:
		case <-ctx.Done():
			return errors.Join(append(failures, fmt.Errorf("world: boss %q preparation cleanup: %w", scope, ctx.Err()))...)
		}
	}
	return errors.Join(failures...)
}

func (bossService *Service) closeBoss(ctx context.Context, state *bossState) error {
	if err := bossService.saveBeforeRelease(ctx, state); err != nil {
		return fmt.Errorf("final save: %w", err)
	}
	released, err := bossService.WorldRegistry.Release(ctx, state.assignment)
	if err != nil {
		return fmt.Errorf("release assignment: %w", err)
	}
	if !released {
		return errors.New("world assignment changed")
	}
	return nil
}

// 同一 Boss 的任务按序执行。循环异常时跳过本次快照，仍等此前已提交的保存。
func (bossService *Service) saveBeforeRelease(ctx context.Context, state *bossState) (err error) {
	defer func() {
		if recover() != nil {
			logging.Panic("boss_final_save")
			err = errors.New("world: final save panic")
		}
		err = database.WaitForSave(ctx, state.lastSave, err)
	}()
	if err = bossService.loop.DomainError(state.assignment.Scope); err != nil {
		return err
	}
	return bossService.save(state)
}

// Boss 只注册 Remote，不提供客户端 Handler；服务认证沿用 NATS 配置。
