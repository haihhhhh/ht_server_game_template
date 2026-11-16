package storage

import (
	"context"

	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"gorm.io/gorm"
)

// RoleInitializer 只在新角色创建事务内执行，缺少模块行的老角色不会重新领到道具。
func RoleInitializer() func(context.Context, *gorm.DB, *model.Role) error {
	return func(_ context.Context, db *gorm.DB, role *model.Role) error {
		// TODO(业务扩展)：新角色必需的初始数据在同一事务中创建，失败就返回错误。
		if err := db.Create(&model.PlayerProfile{RoleID: role.ID}).Error; err != nil {
			return err
		}
		if err := db.Create(&model.PlayerChallenge{
			RoleID:    role.ID,
			Remaining: 3,
		}).Error; err != nil {
			return err
		}
		return db.Create(&model.PlayerItem{
			RoleID: role.ID,
			ItemID: gamedata.StarterExpItemID,
			Count:  5,
		}).Error
	}
}
