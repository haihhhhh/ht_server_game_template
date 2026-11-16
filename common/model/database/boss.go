package database

// WorldBoss 只由公共服务保存，不挂在某个玩家的数据下面。
type WorldBoss struct {
	Scope string `gorm:"primaryKey;type:varchar(256) CHARACTER SET ascii COLLATE ascii_bin"`
	HP    int64  `gorm:"not null"`
}
type PlayerChallenge struct {
	RoleID    int64 `gorm:"primaryKey;autoIncrement:false"`
	Remaining int64 `gorm:"not null"`
}
