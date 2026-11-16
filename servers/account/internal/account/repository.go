package account

import (
	"context"
	"errors"

	"github.com/haihhhhh/ht_server_frame/idgen"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type repository struct {
	IDGenerator  *idgen.Generator
	DB           *gorm.DB
	BeforeCreate func(context.Context, Identity) error
}

// Find 只查询；不存在时返回 gorm.ErrRecordNotFound，不创建账号。
func (accountRepository *repository) Find(ctx context.Context, id Identity) (*model.Account, error) {
	if !valid(id) {
		return nil, ErrInvalidIdentity
	}
	var value model.Account
	err := accountRepository.DB.WithContext(ctx).Where("channel = ? AND subject = ?", id.Channel, id.Subject).First(&value).Error
	return &value, err
}

// Create 使用数据库唯一键防止并发创建重复账号，冲突时返回已有账号。
func (accountRepository *repository) Create(ctx context.Context, id Identity) (*model.Account, error) {
	if !valid(id) {
		return nil, ErrInvalidIdentity
	}
	// 重复创建返回原账号；准入只针对尚不存在的身份，不改变普通查询语义。
	if existing, err := accountRepository.Find(ctx, id); err == nil {
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if accountRepository.BeforeCreate != nil {
		if err := accountRepository.BeforeCreate(ctx, id); err != nil {
			return nil, err
		}
	}
	if accountRepository.IDGenerator == nil {
		return nil, errors.New("账号发号器未配置")
	}
	accountID, err := accountRepository.IDGenerator.Next()
	if err != nil {
		return nil, err
	}
	value := model.Account{
		ID:      accountID,
		Channel: id.Channel,
		Subject: id.Subject,
		Status:  model.Normal,
	}
	if err = accountRepository.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&value).Error; err != nil {
		return nil, err
	}
	return accountRepository.Find(ctx, id)
}
