// TODO(业务扩展)【选区选角分支】提供逻辑区与角色目录；这里查询的是目录数据，不代替 game 中的在线角色组件。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package directory

import (
	"context"

	"github.com/haihhhhh/ht_server_frame/access"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"gorm.io/gorm"
)

type Service struct {
	DB    *database.Service
	Zones []Zone
}

func (directoryService *Service) CheckZone(id string, entering bool) error {
	for _, z := range directoryService.Zones {
		if z.ID == id && (!entering || z.Open) {
			return nil
		}
	}
	return access.ErrForbidden
}

// Roles 每次读数据库，不以在线缓存代替目录刷新。单区上限与角色创建一致。
func (directoryService *Service) Roles(ctx context.Context, accountID int64, zone string, roleID int64) ([]model.Role, error) {
	if accountID <= 0 || roleID < 0 {
		return nil, access.ErrForbidden
	}
	if err := directoryService.CheckZone(zone, false); err != nil {
		return nil, err
	}
	values, err := database.Query(ctx, directoryService.DB, database.AccountKey(accountID), 256, func(db *gorm.DB) ([]model.Role, error) {
		var values []model.Role
		query := db.Where("account_id = ? AND zone = ?", accountID, zone)
		if roleID != 0 {
			query = query.Where("id = ?", roleID)
		}
		err := query.Order("created_at,id").Limit(101).Find(&values).Error
		return values, err
	})
	if err != nil {
		return nil, err
	}
	if len(values) > 100 {
		return nil, access.ErrForbidden
	}
	return values, nil
}
