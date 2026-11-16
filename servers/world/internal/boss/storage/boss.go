// storage 通过 DB 队列加载和保存公共 Boss，调用方持有内存状态。
package storage

import (
	"context"
	"errors"

	"github.com/haihhhhh/ht_server_frame/observability"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Load(ctx context.Context, dbService *database.Service, scope string) (model.WorldBoss, error) {
	return database.Query(ctx, dbService, scope, 256, func(db *gorm.DB) (model.WorldBoss, error) {
		value := model.WorldBoss{
			Scope: scope,
			HP:    1000,
		}
		err := db.First(&value, "scope = ?", scope).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
		}
		return value, err
	})
}

// Save 的参数是值副本，异步写库不会再读取 Boss 循环中的内存。
func Save(dbService *database.Service, value model.WorldBoss) (<-chan error, error) {
	batch := observability.RequestContext(context.Background())
	log := observability.Logger(batch).With("component", "world-boss", "stage", "save")
	done, err := dbService.Submit(batch, value.Scope, 512, func(db *gorm.DB) error {
		err := db.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"hp"})}).Create(&value).Error
		if err != nil {
			log.Error("公共状态写入失败", "kind", "database_error")
		} else {
			log.Debug("公共状态保存执行完成")
		}
		return err
	})
	if err != nil {
		log.Error("公共状态保存未入队", "kind", "queue_rejected")
	}
	return done, err
}
