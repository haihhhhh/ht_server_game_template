// 【通常无需修改｜R07】将具体请求找到的处理函数交给 PlayerManager；普通新协议通过模块登记，无需在此加分支。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package handler

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
)

// playerRequest 在身份和归属核对后，将普通协议交给该玩家的循环。
func (playerSessionRPC *PlayerSessionRPC) playerRequest(ctx context.Context, accountID int64, request proto.Message) (proto.Message, error) {
	size, run := modules.Prepare(request)
	result, err := playerSessionRPC.PlayerManager.Call(ctx, accountID, size, func(task context.Context, onlinePlayer *player.Player) (any, error) {
		return onlinePlayer.RunAction(task, run)
	})
	if err != nil {
		return nil, err
	}
	return result.(proto.Message), nil
}
