package inventory

import (
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules/profile"
)

// TODO(业务扩展)：恢复函数返回的组件会自动参加生命周期和保存；必须返回完整组件；缺失或依赖错误会让玩家加载失败。
var Feature = modules.Register(modules.Definition{
	Name:  "inventory",
	After: "profile",
}, restoreInventory)

func restoreInventory(components *modules.PlayerComponents, role model.Role, data *modules.LoadData) (*Module, error) {
	profileModule, err := profile.Feature.Get(components)
	if err != nil {
		return nil, err
	}
	bag := New(role.ID, profileModule)
	bag.config = &components.Config
	if err = bag.Restore(data.Items); err != nil {
		return nil, err
	}

	return bag, nil
}
