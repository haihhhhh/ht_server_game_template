package player

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
)

// push 只在玩家循环内调用；连接替换也在该循环中执行，因此本次目标不会中途改投。
func (playerManager *PlayerManager) push(ctx context.Context, onlinePlayer *Player, message proto.Message) error {
	account := onlinePlayer.profile.Role().AccountID
	playerManager.mu.RLock()
	entry := playerManager.onlinePlayersByAccount[account]
	binding, bound := playerManager.bindingsByAccount[account]
	valid := bound && !binding.retired && !playerManager.closing && entry != nil && entry.state == onlinePlayer && !entry.loading && !entry.leaving.Load()
	playerManager.mu.RUnlock()
	if !valid {
		return ErrBusy
	}
	packet, err := clusterlink.EncodePush(binding.connection, message)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Tick 没有客户端请求身份；使用刚读取的绑定，同时保留原请求的取消和期限。
	ctx, err = access.WithRemoteIdentity(ctx, binding.connection.Identity)
	if err != nil {
		return err
	}
	return playerManager.App.RPCTo(ctx, binding.connection.Gateway, "gateway.ConnectionPush.Push", new(wrappers.BytesValue), packet)
}
