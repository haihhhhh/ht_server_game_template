package account

import (
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
)

// Account 只在自己的循环中访问；对外返回值副本，不暴露缓存指针。
type Account struct {
	data            model.Account
	binding         clusterlink.Binding
	submittedStatus string // 初次加载的值，后续只在保存成功入队时更新。
	lastSave        <-chan error
}

func (onlineAccount *Account) Data() model.Account {
	return onlineAccount.data
}

func (onlineAccount *Account) Check(binding clusterlink.Binding) error {
	if onlineAccount.binding != binding {
		return access.ErrUnauthenticated
	}
	if onlineAccount.data.Status != model.Normal {
		return ErrBanned
	}
	return nil
}

// SetStatus 供已完成权限检查的业务或管理入口在账号循环中调用。
// TODO(业务扩展)：在账号循环内修改字段；保存时与该字段上次加载或提交的值比较。
func (onlineAccount *Account) SetStatus(status string) error {
	if status != model.Normal && status != model.Banned {
		return ErrInvalidIdentity
	}
	onlineAccount.data.Status = status
	return nil
}
