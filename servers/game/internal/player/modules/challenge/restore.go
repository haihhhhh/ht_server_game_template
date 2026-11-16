package challenge

import (
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
)

// TODO(业务扩展)：恢复函数返回的组件会自动参加生命周期和保存；必须返回完整组件；缺失或依赖错误会让玩家加载失败。
var Feature = modules.Register(modules.Definition{
	Name:  "challenge",
	After: "online_stats",
}, restoreChallenge)

func restoreChallenge(components *modules.PlayerComponents, role model.Role, data *modules.LoadData) (*Module, error) {
	challenges := New(role.ID)
	challenges.world = components.World
	challenges.zone = role.Zone
	challenges.Restore(data.Challenge, data.ChallengeMissing)

	return challenges, nil
}
