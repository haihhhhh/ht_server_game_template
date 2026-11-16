// 【通常无需修改｜进入游戏】把已经查出的数据交给各组件恢复，全部 AfterLoad 成功后返回组件集合；此处不查询数据库。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
// modules 管理玩家组件的装配和生命周期。具体业务包只向这里登记，公共代码不导入业务包。
package modules

import (
	"fmt"
	"time"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_frame/tables"
	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
)

// World 是玩家侧调用 world 服务需要的依赖。
type World struct {
	App      *app.App
	Registry *placement.Registry
}

// PlayerTime 只提供一个业务日期；在线多久是时长，不能拿来当日期。
type PlayerTime struct {
	BusinessNow    func() time.Time
	OnlineDuration func() time.Duration
}

// PlayerComponents 保存一个玩家的业务组件及其调用公共服务所需的资源。
type PlayerComponents struct {
	Manager
	PlayerTime
	World World
	// 每个请求开始时更新一次；组件共享这个位置，整次请求使用同一版配置。
	Config       gamedata.Data
	configSource *tables.Module[gamedata.Data]
	instances    []Module
}

func Load(role model.Role, data *LoadData, world World, configSource *tables.Module[gamedata.Data], playerTime PlayerTime) (*PlayerComponents, error) {
	if playerTime.BusinessNow == nil || playerTime.OnlineDuration == nil {
		return nil, fmt.Errorf("玩家组件缺少业务时钟或在线计时器")
	}
	order, err := ordered()
	if err != nil {
		return nil, err
	}
	components := &PlayerComponents{
		World:        world,
		PlayerTime:   playerTime,
		configSource: configSource,
		instances:    make([]Module, len(order)),
	}
	if err = components.RefreshConfig(); err != nil {
		return nil, err
	}
	for _, f := range order {
		module, err := f.restore(components, role, data)
		if err != nil {
			return nil, fmt.Errorf("恢复玩家组件 %s: %w", f.Name, err)
		}
		components.instances[f.slot] = module
		components.Modules = append(components.Modules, module)
		if handler, ok := module.(tickHandler); ok {
			components.tickHandlers = append(components.tickHandlers, handler)
		}
	}
	if err = components.AfterLoad(); err != nil {
		return nil, err
	}
	return components, nil
}

// RefreshConfig 只在所属玩家循环内调用；每次只替换只读入口，背包和角色共用此地址。
func (components *PlayerComponents) RefreshConfig() error {
	if components.configSource == nil {
		return fmt.Errorf("玩家缺少配置表资源")
	}
	snapshot, err := components.configSource.Snapshot()
	if err != nil {
		return err
	}
	components.Config = snapshot.Value()
	return nil
}
