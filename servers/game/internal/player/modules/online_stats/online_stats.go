// TODO(业务扩展)【生命周期示例】通过 Enter、Tick、Leave 统计进入次数与真实在线时长；无需客户端请求，定期保存走同一条 S01–S04 流程。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package onlinestats

import (
	"errors"
	"time"

	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Module struct {
	data           model.PlayerOnlineStats
	acceptedData   model.PlayerOnlineStats
	onlineDuration func() time.Duration
	secondsAtEnter int64
}

func (*Module) Name() string {
	return "online_stats"
}
func (onlineStatsModule *Module) AfterLoad() error {
	if onlineStatsModule.data.RoleID <= 0 || onlineStatsModule.data.LoginCount < 0 || onlineStatsModule.data.OnlineSeconds < 0 {
		return errors.New("invalid online statistics data")
	}
	return nil
}

func (onlineStatsModule *Module) Restore(data model.PlayerOnlineStats, missing bool) {
	onlineStatsModule.data = data
	onlineStatsModule.acceptedData = model.PlayerOnlineStats{}
	if !missing {
		onlineStatsModule.acceptedData = data
	}
}
func (onlineStatsModule *Module) Enter(_ time.Time) {
	onlineStatsModule.data.LoginCount++
	onlineStatsModule.secondsAtEnter = onlineStatsModule.data.OnlineSeconds
}
func (onlineStatsModule *Module) Tick(_ time.Time) {
	onlineSeconds := int64(onlineStatsModule.onlineDuration() / time.Second)
	onlineStatsModule.data.OnlineSeconds = onlineStatsModule.secondsAtEnter + onlineSeconds
}
func (onlineStatsModule *Module) Leave(now time.Time) {
	onlineStatsModule.Tick(now)
}
func (onlineStatsModule *Module) Accepted() {
	onlineStatsModule.acceptedData = onlineStatsModule.data
}
func (onlineStatsModule *Module) Snapshot() *modules.Save {
	value := onlineStatsModule.data
	previous := onlineStatsModule.acceptedData
	if previous == value {
		return nil
	}
	var columns []string
	if previous.RoleID == 0 || previous.LoginCount != value.LoginCount {
		columns = append(columns, "login_count")
	}
	if previous.RoleID == 0 || previous.OnlineSeconds != value.OnlineSeconds {
		columns = append(columns, "online_seconds")
	}
	return &modules.Save{
		Write: func(db *gorm.DB) error {
			return db.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns(columns)}).Create(&value).Error
		},
	}
}

func New(roleID int64, onlineDuration func() time.Duration) *Module {
	return &Module{data: model.PlayerOnlineStats{RoleID: roleID}, onlineDuration: onlineDuration}
}
func (onlineStatsModule *Module) Data() model.PlayerOnlineStats {
	return onlineStatsModule.data
}
