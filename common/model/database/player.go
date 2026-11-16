package database

// 各类记录由 Models 统一建表；这里不声明业务组件之间的依赖。
type PlayerProfile struct {
	RoleID      int64  `gorm:"primaryKey;autoIncrement:false"`
	Preferences string `gorm:"type:text;not null"`
	Experience  int64  `gorm:"not null;default:0"`
}

type PlayerItem struct {
	RoleID int64 `gorm:"primaryKey;autoIncrement:false"`
	ItemID int32 `gorm:"primaryKey;autoIncrement:false"`
	Count  int64 `gorm:"not null"`
}

type PlayerOnlineStats struct {
	RoleID        int64 `gorm:"primaryKey;autoIncrement:false"`
	LoginCount    int64 `gorm:"not null"`
	OnlineSeconds int64 `gorm:"not null;default:0"`
}

// TableName 保留已有表名，模块改名不迁移或丢弃历史在线统计。
func (PlayerOnlineStats) TableName() string {
	return "player_activities"
}
