package profile

import (
	"errors"
	"time"

	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Module struct {
	config *gamedata.Data
	// 仅显式整体同步使用；恢复时绑定当前玩家的汇总方法，不在普通请求中调用。
	currentState func() *pb.PlayerState
	data         model.PlayerProfile
	acceptedData model.PlayerProfile
	role         model.Role
	acceptedName string
}

func (*Module) Name() string {
	return "profile"
}
func (profileModule *Module) AfterLoad() error {
	if profileModule.currentState == nil || profileModule.config == nil {
		return errors.New("profile: missing state snapshot dependency")
	}
	if profileModule.role.ID != profileModule.data.RoleID || profileModule.role.AccountID <= 0 || profileModule.role.Zone == "" || profileModule.role.Status != model.Normal {
		return errors.New("invalid role data")
	}
	if profileModule.data.RoleID <= 0 || len(profileModule.data.Preferences) > maxPreferencesBytes || profileModule.data.Experience < 0 || profileModule.data.Experience > MaxExperience {
		return errors.New("invalid profile data")
	}
	return nil
}

func (profileModule *Module) Restore(data model.PlayerProfile, missing bool) {
	profileModule.data = data
	profileModule.acceptedData = model.PlayerProfile{}
	if !missing {
		profileModule.acceptedData = data
	}
}
func (*Module) Enter(time.Time) {}
func (*Module) Leave(time.Time) {}
func (profileModule *Module) Accepted() {
	profileModule.acceptedData = profileModule.data
	profileModule.acceptedName = profileModule.role.Name
}
func (profileModule *Module) Snapshot() *modules.Save {
	previous := profileModule.acceptedData
	data := profileModule.data
	name := profileModule.role.Name
	saveName := name != profileModule.acceptedName
	if data == previous && !saveName {
		return nil
	}
	// 缺少资料行时补齐初值，之后只修改实际变化的列。
	missing := previous.RoleID == 0
	value := model.PlayerProfile{RoleID: data.RoleID}
	var columns []string
	if missing || data.Preferences != previous.Preferences {
		value.Preferences = data.Preferences
		columns = append(columns, "preferences")
	}
	if missing || data.Experience != previous.Experience {
		value.Experience = data.Experience
		columns = append(columns, "experience")
	}
	return &modules.Save{
		ExtraBytes: len(value.Preferences) + len(name),
		Write: func(db *gorm.DB) error {
			var failures []error
			if len(columns) != 0 {
				failures = append(failures, db.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns(columns)}).Create(&value).Error)
			}
			if saveName {
				// 只保存在线业务可修改的名称，不能覆盖外部管理的封禁和归属字段。
				failures = append(failures, db.Model(&model.Role{}).Where("id = ?", value.RoleID).Update("name", name).Error)
			}
			return errors.Join(failures...)
		},
	}
}

func New(roleID int64) *Module {
	return &Module{data: model.PlayerProfile{RoleID: roleID}}
}
func (profileModule *Module) Data() model.PlayerProfile {
	return profileModule.data
}
