// 【通常无需修改｜S02】收集各组件的独立保存副本，作为同账号 DB 任务提交；入队成功不等于数据库已保存成功。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package storage

import (
	"context"
	"errors"

	"github.com/haihhhhh/ht_server_frame/observability"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"gorm.io/gorm"
)

// 每份组件快照的固定队列估值，变长数据的占用由组件另计。
const snapshotBaseBytes int64 = 128

// Saver 属于一个在线玩家。没有新变化或入队失败时，退出流程仍能等到上次提交的结果。
type Saver struct {
	LastSubmitted <-chan error
}

// SubmitChanges 在定期保存和下线时都只收集变化；final 仅用于标记下线日志。
func (saver *Saver) SubmitChanges(dbService *database.Service, accountID, roleID int64, playerModules []modules.Module, final bool) error {
	jobs := make([]*modules.Save, 0, len(playerModules))
	changed := make([]modules.Module, 0, len(playerModules))
	names := make([]string, 0, len(playerModules))
	var size int64
	for _, module := range playerModules {
		if job := module.Snapshot(); job != nil {
			jobs = append(jobs, job)
			changed = append(changed, module)
			names = append(names, module.Name())
			size += snapshotBaseBytes + int64(max(0, job.ExtraBytes))
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	batch := observability.RequestContext(context.Background())
	log := observability.Logger(batch).With("role_id", roleID, "final", final)
	done, err := dbService.Submit(batch, database.AccountKey(accountID), size, func(db *gorm.DB) error {
		var failures []error
		for i, job := range jobs {
			if err := job.Write(db); err != nil {
				failures = append(failures, err)
				log.Error("玩家组件保存失败", "component", names[i], "stage", "write", "kind", "database_error")
			}
		}
		if len(failures) > 0 {
			log.Error("玩家模块保存失败", "component", "player", "stage", "write", "failed_modules", len(failures))
			return errors.New("Player: module save failed")
		}
		log.Debug("玩家保存执行完成", "component", "player", "stage", "write", "modules", len(jobs))
		return nil
	})
	if err != nil {
		log.Error("玩家保存未入队", "component", "player", "stage", "enqueue", "kind", "queue_rejected")
		return err
	}
	saver.LastSubmitted = done
	log.Debug("玩家保存已入队", "component", "player", "stage", "enqueue", "modules", len(jobs))
	for _, module := range changed {
		module.Accepted()
	}
	return nil
}
