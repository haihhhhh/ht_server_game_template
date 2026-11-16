package challenge

import (
	"errors"
	"time"

	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Module 保存个人挑战次数及已装配的 world 调用依赖，不包含每日重置业务。
type Module struct {
	world modules.World
	zone  string
	data  model.PlayerChallenge
	dirty bool
}

func (*Module) Name() string {
	return "challenge"
}
func (challengeModule *Module) AfterLoad() error {
	if challengeModule.world.App == nil || challengeModule.world.Registry == nil || challengeModule.zone == "" {
		return errors.New("challenge: missing world dependencies or zone")
	}
	if challengeModule.data.RoleID <= 0 || challengeModule.data.Remaining < 0 {
		return errors.New("invalid challenge data")
	}
	return nil
}

func (challengeModule *Module) Restore(data model.PlayerChallenge, missing bool) {
	challengeModule.data = data
	challengeModule.dirty = missing
}
func (*Module) Enter(time.Time) {}
func (*Module) Leave(time.Time) {}
func (challengeModule *Module) Accepted() {
	challengeModule.dirty = false
}
func (challengeModule *Module) Snapshot() *modules.Save {
	if !challengeModule.dirty {
		return nil
	}
	value := challengeModule.data
	return &modules.Save{
		Write: func(db *gorm.DB) error {
			return db.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"remaining"})}).Create(&value).Error
		},
	}
}

func New(roleID int64) *Module {
	return &Module{data: model.PlayerChallenge{RoleID: roleID}}
}
func (challengeModule *Module) Remaining() int64 {
	return challengeModule.data.Remaining
}
func (challengeModule *Module) Consume() {
	challengeModule.data.Remaining--
	challengeModule.dirty = true
}
