// 【通常无需修改｜进入游戏】选定角色、集中加载数据、恢复组件后才开放业务；普通玩家请求不会重复走这条加载流程。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package player

import (
	"context"

	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	_ "github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules/profile"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/storage"
)

func (playerManager *PlayerManager) load(ctx context.Context, account int64, zone string, roleID int64, onlinePlayer *Player) error {
	selected, err := playerManager.RoleService.Enter(ctx, account, zone, roleID)
	if err != nil {
		return err
	}
	data, err := storage.Load(ctx, playerManager.DB, account, selected.ID)
	if err != nil {
		return err
	}
	components, err := modules.Load(
		*selected,
		data,
		modules.World{App: playerManager.App, Registry: playerManager.WorldRegistry},
		playerManager.ConfigTables,
		modules.PlayerTime{
			BusinessNow:    playerManager.Clock.Now,
			OnlineDuration: onlinePlayer.onlineDuration,
		},
	)
	if err != nil {
		return err
	}
	profileModule, err := profile.Feature.Get(components)
	if err != nil {
		return err
	}
	*onlinePlayer = Player{
		playerManager: playerManager,
		profile:       profileModule,
		components:    components,
	}
	onlinePlayer.startTimers()
	onlinePlayer.components.Enter(playerManager.Clock.Now())
	onlinePlayer.ready = true
	return nil
}

func (playerManager *PlayerManager) save(onlinePlayer *Player, final bool) error {
	role := onlinePlayer.profile.Role()
	return onlinePlayer.saver.SubmitChanges(playerManager.DB, role.AccountID, role.ID, onlinePlayer.components.Modules, final)
}
