package database

import "time"

const (
	Normal = "normal"
	Banned = "banned"
)

// Account 只关联验证过的渠道身份；账号与角色分别分配 ID。
type Account struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false"`
	Channel   string    `gorm:"size:32;not null;uniqueIndex:channel_subject"`
	Subject   string    `gorm:"size:191;not null;uniqueIndex:channel_subject"`
	Status    string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"precision:3;not null"`
}
