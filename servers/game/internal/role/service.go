package role

import (
	"context"

	"github.com/haihhhhh/ht_server_frame/idgen"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"gorm.io/gorm"
)

type Service struct {
	IDGenerator *idgen.Generator
	DB          *database.Service
	MaxPerZone  int
	// Initialize 在数据库创建事务内执行，不得再次提交数据库队列或调用外部服务。
	Initialize func(context.Context, *gorm.DB, *model.Role) error
}

func (roleService *Service) repository(db *gorm.DB) *repository {
	return &repository{
		DB:          db,
		IDGenerator: roleService.IDGenerator,
		MaxPerZone:  roleService.MaxPerZone,
		Initialize:  roleService.Initialize,
	}
}

func (roleService *Service) List(ctx context.Context, account int64, zone string) ([]model.Role, error) {
	return database.Query(ctx, roleService.DB, database.AccountKey(account), int64(len(zone))+256, func(db *gorm.DB) ([]model.Role, error) {
		return roleService.repository(db).List(db.Statement.Context, account, zone)
	})
}

func (roleService *Service) Get(ctx context.Context, account int64, zone string, id int64) (*model.Role, error) {
	return database.Query(ctx, roleService.DB, database.AccountKey(account), int64(len(zone))+256, func(db *gorm.DB) (*model.Role, error) {
		return roleService.repository(db).Get(db.Statement.Context, account, zone, id)
	})
}

func (roleService *Service) Create(ctx context.Context, account int64, zone, name string) (*model.Role, error) {
	return database.Query(ctx, roleService.DB, database.AccountKey(account), int64(len(zone)+len(name))+256, func(db *gorm.DB) (*model.Role, error) {
		return roleService.repository(db).Create(db.Statement.Context, account, zone, name)
	})
}

func (roleService *Service) Enter(ctx context.Context, account int64, zone string, id int64) (*model.Role, error) {
	return database.Query(ctx, roleService.DB, database.AccountKey(account), int64(len(zone))+256, func(db *gorm.DB) (*model.Role, error) {
		return roleService.repository(db).Enter(db.Statement.Context, account, zone, id)
	})
}
