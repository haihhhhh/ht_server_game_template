package database

import "time"

// Role 是持久保存的角色档案。一个账号可以有多个角色，离线时档案仍然存在。
type Role struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false"`
	AccountID int64     `gorm:"not null;index:account_zone"`
	Zone      string    `gorm:"size:64;not null;index:account_zone"`
	Name      string    `gorm:"size:64;not null"`
	Status    string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"precision:3;not null"`
}
