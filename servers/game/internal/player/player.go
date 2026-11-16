// 【通常无需修改｜R09】Player 持有一个角色的在线组件，把排队完成的动作交给 PlayerComponents；不在这里堆放具体玩法。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package player

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules/profile"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/storage"
)

// Player 是账号进入某个角色后，game 在内存中管理的在线对象，持有该角色的背包等业务组件。
// 其业务字段只由所属循环读写，循环结束后才允许退出流程收集最后副本。
type Player struct {
	playerManager *PlayerManager
	profile       *profile.Module
	ready         bool
	enteredAt     time.Time
	nextSave      time.Duration
	nextTick      time.Duration
	components    *modules.PlayerComponents
	saver         storage.Saver
}

// Push 只能在玩家循环中调用，按管理器当前绑定定向推送，不按账号广播。
// 成功仅表示交给发送流程，不保证客户端在线或收到，也不自动重试。
func (onlinePlayer *Player) Push(ctx context.Context, message proto.Message) error {
	if !onlinePlayer.ready || onlinePlayer.playerManager == nil {
		return ErrBusy
	}
	return onlinePlayer.playerManager.push(ctx, onlinePlayer, message)
}

// RunAction 只在玩家循环内调用，业务动作仅接收该玩家的组件，不持有管理器。
func (onlinePlayer *Player) RunAction(ctx context.Context, run modules.Action) (proto.Message, error) {
	if err := onlinePlayer.components.RefreshConfig(); err != nil {
		return nil, err
	}
	return run(ctx, onlinePlayer.components)
}
