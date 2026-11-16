package storage

import (
	"context"
	"errors"
	"fmt"

	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules/inventory"
	"gorm.io/gorm"
)

// loadPlayerData 在一个 DB 队列任务中顺序查询，不把多行背包与其他表做大 JOIN。
func loadPlayerData(db *gorm.DB, roleID int64) (*modules.LoadData, error) {
	data := &modules.LoadData{
		Profile:     model.PlayerProfile{RoleID: roleID},
		OnlineStats: model.PlayerOnlineStats{RoleID: roleID},
		Challenge:   model.PlayerChallenge{RoleID: roleID},
	}
	type loadRow struct {
		name    string
		value   any
		missing *bool
	}
	// TODO(业务扩展)：新组件的数据在本次加载任务中读取；多行数据参照下方背包查询。
	rows := []loadRow{
		{"profile", &data.Profile, &data.ProfileMissing},
		{"OnlineStats", &data.OnlineStats, &data.OnlineStatsMissing},
	}
	rows = append(rows, loadRow{"challenge", &data.Challenge, &data.ChallengeMissing})
	for _, row := range rows {
		err := db.First(row.value, "role_id = ?", roleID).Error
		*row.missing = errors.Is(err, gorm.ErrRecordNotFound)
		// 老角色缺失模块保持零值，不重复补发新手奖励。
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("load %s: %w", row.name, err)
		}
	}
	if err := db.Where("role_id = ?", roleID).Limit(inventory.MaxKinds + 1).Find(&data.Items).Error; err != nil {
		return nil, fmt.Errorf("load inventory: %w", err)
	}
	return data, nil
}

func Load(ctx context.Context, dbService *database.Service, account, roleID int64) (*modules.LoadData, error) {
	return database.Query(ctx, dbService, database.AccountKey(account), 256, func(db *gorm.DB) (*modules.LoadData, error) {
		return loadPlayerData(db, roleID)
	})
}
