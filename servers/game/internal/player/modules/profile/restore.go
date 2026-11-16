package profile

import (
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
)

// TODO(业务扩展)：恢复函数返回的组件会自动参加生命周期和保存；必须返回完整组件；缺失或依赖错误会让玩家加载失败。
var Feature = modules.Register(modules.Definition{
	Name:  "profile",
	After: "",
}, restoreProfile)

func restoreProfile(components *modules.PlayerComponents, role model.Role, data *modules.LoadData) (*Module, error) {
	roleProfile := New(role.ID)
	roleProfile.config = &components.Config
	roleProfile.currentState = components.Current
	roleProfile.Restore(data.Profile, data.ProfileMissing)
	roleProfile.RestoreRole(role)

	return roleProfile, nil
}
