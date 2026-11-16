package modules

import (
	"time"

	"gorm.io/gorm"
)

// Module 的方法只在玩家循环或循环结束后执行。业务扩展：新增玩家组件参照 inventory.Module 实现此接口，再参照 player/modules/inventory/restore.go 声明装配。
// Snapshot 必须返回独立副本，不能让数据库任务引用正在变化的模块字段。
type Module interface {
	// 名称用于日志；应与本组件登记的名称一致。
	Name() string
	// TODO(业务扩展)：全部组件恢复后检查数据；失败就不允许玩家进入。
	AfterLoad() error
	// TODO(业务扩展)：角色进入时执行一次，传入当前业务时间；新连接替换旧连接不重复执行。
	Enter(businessNow time.Time)
	// TODO(业务扩展)：离开时按业务时间结算，不在这里再次等待自己的玩家循环。
	Leave(businessNow time.Time)
	// TODO(业务扩展)【S02、S04】：只复制上次入队后发生的变化，无变化时返回 nil。定期保存和下线共用此方法。
	Snapshot() *Save
	// 通常只清除本次变化标记：表示保存任务已入队，不表示写库成功。
	Accepted()
}

// TODO(业务扩展)：需要定时执行的组件实现 Tick(time.Time)，装配后会在玩家循环中自动调用。
type tickHandler interface {
	Tick(businessNow time.Time)
}

type Save struct {
	Write func(*gorm.DB) error
	// ExtraBytes 只估算文本、批量条目等变长数据占用的字节。普通快照只需提供 Write。
	ExtraBytes int
}
