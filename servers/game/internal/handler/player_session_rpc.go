// 【通常无需修改｜R06】game 的统一 RPC 入口：核对账号、节点归属和连接，再区分进入、退出与普通业务请求。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package handler

import (
	"context"
	"sync"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player"
	"github.com/topfreegames/pitaya/v2/component"
)

type owned struct {
	mu          sync.Mutex
	assignment  placement.Assignment
	retired     bool
	exitBinding clusterlink.Binding
	exitDone    chan struct{}
	saved       bool
	exitErr     error
}

// PlayerSessionRPC 接收网关的玩家请求。每账号的锁只协调绑定、替换和退出，
// 进入角色和普通玩法在核对归属后交给 PlayerManager。
type PlayerSessionRPC struct {
	component.Base
	App           *app.App
	PlayerManager *player.PlayerManager
	GameRegistry  *placement.Registry
	mu            sync.Mutex
	users         map[int64]*owned
	closing       bool
}

func (playerSessionRPC *PlayerSessionRPC) Dispatch(ctx context.Context, requestPacket *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.CallRemote(ctx, playerSessionRPC.App, "Cluster.Dispatch", false, requestPacket, func(ctx context.Context, requestPacket *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		out, err := playerSessionRPC.receive(ctx, requestPacket, false)
		return out, publicError(err)
	})
}
func (playerSessionRPC *PlayerSessionRPC) Closed(ctx context.Context, requestPacket *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.CallRemote(ctx, playerSessionRPC.App, "Cluster.Closed", true, requestPacket, func(ctx context.Context, requestPacket *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		out, err := playerSessionRPC.receive(ctx, requestPacket, true)
		return out, publicError(err)
	})
}

func (playerSessionRPC *PlayerSessionRPC) receive(ctx context.Context, requestPacket *wrappers.BytesValue, closed bool) (*wrappers.BytesValue, error) {
	id, err := access.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	accountID, err := clusterlink.ParseAccountID(id.UserID)
	if err != nil {
		return nil, err
	}
	var command clusterlink.Command
	if clusterlink.Decode(requestPacket, &command) != nil || command.Gateway == "" || len(command.Gateway) > 256 || command.Allocation == "" || len(command.Allocation) > 128 || len(command.Token) < 16 || len(command.Token) > 128 {
		return nil, access.ErrForbidden
	}
	if !command.Action.IsGame() || closed != (command.Action == clusterlink.Disconnect) || (command.Claim && command.Action != clusterlink.Attach) {
		return nil, access.ErrForbidden
	}
	request, err := command.Request()
	if err != nil {
		return nil, err
	}
	incoming := clusterlink.Binding{
		Gateway:  command.Gateway,
		Identity: id,
		Token:    command.Token,
		Order:    command.Order,
	}
	if !incoming.Order.Valid() {
		return nil, access.ErrForbidden
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	switch command.Action {
	case clusterlink.Attach:
		return playerSessionRPC.attach(ctx, accountID, command, incoming)
	case clusterlink.Leave, clusterlink.Disconnect:
		entry, exitErr := playerSessionRPC.beginExit(ctx, accountID, command, incoming)
		if exitErr != nil {
			return nil, exitErr
		}
		if entry == nil {
			return clusterlink.Encode(clusterlink.Reply{})
		}
		return playerSessionRPC.waitExit(ctx, entry)
	}

	owner, err := playerSessionRPC.PlayerManager.RequestOwner(accountID, incoming)
	if err != nil {
		return nil, err
	}
	requestContext := player.WithBinding(ctx, incoming, owner)
	if command.Action == clusterlink.Enter {
		if _, err = playerSessionRPC.checkOwner(ctx, id.UserID, command.Allocation, owner); err != nil {
			return nil, err
		}
		state, enterErr := playerSessionRPC.PlayerManager.Enter(requestContext, accountID, request.(*pb.EnterReq))
		if enterErr != nil {
			return nil, enterErr
		}
		return clusterlink.EncodeReply(&pb.EnterRep{State: state})
	}
	// 已进入角色的玩法只认本机绑定。玩家循环在入队和执行前都用这份快照复核。
	if owner.Scope != id.UserID || owner.AllocationID != command.Allocation || owner.ServerID != playerSessionRPC.App.ServerID() {
		return nil, access.ErrForbidden
	}
	result, err := playerSessionRPC.playerRequest(requestContext, accountID, request)
	if err != nil {
		return nil, err
	}
	return clusterlink.EncodeReply(result)
}

// checkOwner 核对目标节点、请求中的分配编号和本地记录的 etcd 版本。
func (playerSessionRPC *PlayerSessionRPC) checkOwner(ctx context.Context, scope, allocationID string, local placement.Assignment) (placement.Assignment, error) {
	assigned, found, err := playerSessionRPC.GameRegistry.Lookup(ctx, scope)
	if err != nil {
		return assigned, err
	}
	if !found || assigned.ServerID != playerSessionRPC.App.ServerID() || assigned.AllocationID != allocationID {
		return assigned, access.ErrForbidden
	}
	if local.AllocationID != "" && local != assigned {
		return assigned, access.ErrForbidden
	}
	return assigned, nil
}
