// TODO(业务扩展)【登录分支】账号认证、注册规则在账号服务维护；已进入游戏的普通道具请求不经过这里。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package account

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/haihhhhh/ht_server_frame/idgen"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"gorm.io/gorm"
)

type Service struct {
	IDGenerator *idgen.Generator
	DB          *database.Service
	Verify      Verifier
	// BeforeCreate 在 DB 队列中执行准入检查；可并发重复调用，不得发奖或再次等待 DB 队列。
	BeforeCreate func(context.Context, Identity) error
}

// 渠道与主体构成创建前的稳定身份；使用结构编码避免字符串拼接碰撞。
func identityKey(id Identity) string {
	b, _ := json.Marshal(id)
	return "identity:" + string(b)
}

func (accountService *Service) Find(ctx context.Context, id Identity) (*model.Account, error) {
	if !valid(id) {
		return nil, ErrInvalidIdentity
	}
	value, err := database.Query(ctx, accountService.DB, identityKey(id), int64(len(id.Channel)+len(id.Subject))+256, func(db *gorm.DB) (*model.Account, error) {
		value, queryErr := (&repository{DB: db}).Find(db.Statement.Context, id)
		if errors.Is(queryErr, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return value, queryErr
	})
	if err == nil && value == nil {
		// 查询已正常完成；在队列外保留 Find 的未命中语义，供登录决定是否创建。
		return nil, gorm.ErrRecordNotFound
	}
	return value, err
}

func (accountService *Service) Create(ctx context.Context, id Identity) (*model.Account, error) {
	if !valid(id) {
		return nil, ErrInvalidIdentity
	}
	return database.Query(ctx, accountService.DB, identityKey(id), int64(len(id.Channel)+len(id.Subject))+256, func(db *gorm.DB) (*model.Account, error) {
		return (&repository{
			DB:           db,
			BeforeCreate: accountService.BeforeCreate,
			IDGenerator:  accountService.IDGenerator,
		}).Create(db.Statement.Context, id)
	})
}

func (accountService *Service) Login(ctx context.Context, credential string) (int64, error) {
	if accountService.Verify == nil {
		return 0, errors.New("account: verifier not configured")
	}
	id, err := accountService.Verify(ctx, credential)
	if err != nil {
		return 0, err
	}
	value, err := accountService.Find(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		value, err = accountService.Create(ctx, id)
	}
	if err != nil {
		return 0, err
	}
	if value.Status != model.Normal {
		return 0, ErrBanned
	}
	return value.ID, nil
}
