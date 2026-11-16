// TODO(业务扩展)【创建角色分支】在 DB 任务中执行角色创建和必需模块初始化；与普通请求后的定期保存是两条不同流程。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package role

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/haihhhhh/ht_server_frame/idgen"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrForbidden = errors.New("role: unavailable")
	ErrLimit     = errors.New("role: limit reached")
	ErrChoose    = errors.New("role: choose a role explicitly")
)

type repository struct {
	IDGenerator *idgen.Generator
	DB          *gorm.DB
	MaxPerZone  int
	// Initialize 在创建事务中写入项目初始数据，不应调用外部服务。
	Initialize func(context.Context, *gorm.DB, *model.Role) error
}

func valid(account int64, zone string) bool {
	return account > 0 && validText(zone, 64, false)
}

func validText(value string, maxBytes int, allowEmpty bool) bool {
	return (allowEmpty || value != "") && len(value) <= maxBytes && utf8.ValidString(value) && strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func (roleRepository *repository) checkAccount(ctx context.Context, account int64) error {
	var owner model.Account
	err := roleRepository.DB.WithContext(ctx).Select("id", "status").First(&owner, "id = ?", account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if owner.Status != model.Normal {
		return ErrForbidden
	}
	return nil
}

func (roleRepository *repository) List(ctx context.Context, account int64, zone string) ([]model.Role, error) {
	if !valid(account, zone) {
		return nil, ErrForbidden
	}
	if err := roleRepository.checkAccount(ctx, account); err != nil {
		return nil, err
	}
	var values []model.Role
	err := roleRepository.DB.WithContext(ctx).Where("account_id = ? AND zone = ?", account, zone).Order("created_at,id").Limit(101).Find(&values).Error
	if len(values) > 100 {
		return nil, ErrLimit
	}
	return values, err
}

func (roleRepository *repository) Get(ctx context.Context, account int64, zone string, id int64) (*model.Role, error) {
	if !valid(account, zone) || id <= 0 {
		return nil, ErrForbidden
	}
	if err := roleRepository.checkAccount(ctx, account); err != nil {
		return nil, err
	}
	var value model.Role
	err := roleRepository.DB.WithContext(ctx).Where("id = ? AND account_id = ? AND zone = ?", id, account, zone).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, err
	}
	if value.Status != model.Normal {
		return nil, ErrForbidden
	}
	return &value, nil
}

// Create 对账号行加锁后检查数量；所有创建入口都经过这里，不能绕过并发上限。
func (roleRepository *repository) Create(ctx context.Context, account int64, zone, name string) (*model.Role, error) {
	return roleRepository.create(ctx, account, zone, name, false)
}

func (roleRepository *repository) create(ctx context.Context, account int64, zone, name string, first bool) (*model.Role, error) {
	if !valid(account, zone) || !validText(name, 64, true) {
		return nil, ErrForbidden
	}
	limit := roleRepository.MaxPerZone
	if limit == 0 {
		limit = 1
	}
	if limit < 1 || limit > 100 {
		return nil, ErrLimit
	}
	var result model.Role
	err := roleRepository.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner model.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&owner, "id = ?", account).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrForbidden
			}
			return err
		}
		if owner.Status != model.Normal {
			return ErrForbidden
		}
		var existing []model.Role
		if err := tx.Where("account_id = ? AND zone = ?", account, zone).Order("created_at,id").Limit(101).Find(&existing).Error; err != nil {
			return err
		}
		if first && len(existing) > 0 {
			if len(existing) != 1 {
				return ErrChoose
			}
			result = existing[0]
			if result.Status != model.Normal {
				return ErrForbidden
			}
			return nil
		}
		if len(existing) >= limit {
			return ErrLimit
		}
		if roleRepository.IDGenerator == nil {
			return errors.New("角色发号器未配置")
		}
		roleID, err := roleRepository.IDGenerator.Next()
		if err != nil {
			return err
		}
		result = model.Role{
			ID:        roleID,
			AccountID: account,
			Zone:      zone,
			Name:      name,
			Status:    model.Normal,
		}
		if err = tx.Create(&result).Error; err != nil {
			return err
		}
		if roleRepository.Initialize != nil {
			return roleRepository.Initialize(ctx, tx, &result)
		}
		return nil
	})
	return &result, err
}

// Enter 无角色时创建首个，有一个直接进入，有多个则要求明确选择。
func (roleRepository *repository) Enter(ctx context.Context, account int64, zone string, id int64) (*model.Role, error) {
	if id != 0 {
		return roleRepository.Get(ctx, account, zone, id)
	}
	return roleRepository.create(ctx, account, zone, "", true)
}
