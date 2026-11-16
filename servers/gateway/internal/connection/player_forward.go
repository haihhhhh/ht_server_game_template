// 【通常无需修改｜R04、R05、R11】读取当前连接与 game 归属，把请求发往指定节点，再取回业务响应。这里不计算道具、经验等规则。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package connection

import (
	"context"
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	perror "github.com/topfreegames/pitaya/v2/errors"
)

func (connectionManager *Manager) attach(ctx context.Context, client *connection, enteringRole bool) (clusterlink.Binding, error) {
	if _, err := access.FromContext(ctx); err != nil {
		return clusterlink.Binding{}, err
	}
	game := &client.slot.game
	alreadyBound := game.confirmed.binding == client.binding
	if alreadyBound && !enteringRole {
		return clusterlink.Binding{}, nil
	}
	if game.pending.assignment.AllocationID != "" {
		return clusterlink.Binding{}, errors.New("login result unconfirmed; reconnect required")
	}
	accountID := client.binding.Identity.UserID
	assigned, found, err := connectionManager.GameRegistry.Lookup(ctx, accountID)
	if err != nil {
		return clusterlink.Binding{}, err
	}
	// Leave 回包超时后，game 可能已经释放归属。显式 Enter 只复用仍是同一次分配的记录。
	if found && alreadyBound {
		previous := game.confirmed.assignment
		if assigned.ServerID != previous.ServerID || assigned.AllocationID != previous.AllocationID {
			return clusterlink.Binding{}, perror.NewError(errors.New("角色归属已变更，请重新登录"), "GAME-RELOGIN-REQUIRED")
		}
		return clusterlink.Binding{}, nil
	}
	if !found {
		game.confirmed = backendTarget{}
		if !enteringRole {
			return clusterlink.Binding{}, nil
		}
		assigned, err = clusterlink.SelectCandidate(connectionManager.App, "game", accountID)
		if err != nil {
			return clusterlink.Binding{}, err
		}
	}
	_, reply, err := attachOwner(assigned, !found, func(target placement.Assignment, claim bool) (clusterlink.Reply, error) {
		candidate := backendTarget{assignment: target, binding: client.binding}
		command := clusterlink.Command{Action: clusterlink.Attach, Claim: claim}
		return connectionManager.sendAttach(ctx, game, candidate, "game.Cluster.Dispatch", command)
	})
	if err != nil {
		return clusterlink.Binding{}, err
	}
	game.confirmed = backendTarget{assignment: *reply.Assignment, binding: client.binding}
	game.pending = backendTarget{}
	return reply.Previous, nil
}

func (connectionManager *Manager) send(ctx context.Context, client *connection, action clusterlink.Action, request proto.Message) (clusterlink.Reply, error) {
	var out clusterlink.Reply
	target := client.slot.game.confirmed
	requestPacket, err := clusterlink.EncodeRequest(clusterlink.Command{
		Allocation: target.assignment.AllocationID,
		Gateway:    target.binding.Gateway,
		Token:      target.binding.Token,
		Order:      target.binding.Order,
		Action:     action,
	}, request)
	if err != nil {
		return out, err
	}
	raw := new(wrappers.BytesValue)
	send := connectionManager.App.RPCTo
	if client.enteredRole {
		send = connectionManager.App.RPCToBoundServer
	}
	err = send(ctx, target.assignment.ServerID, "game.Cluster.Dispatch", raw, requestPacket)
	if err != nil {
		return out, err
	}
	err = clusterlink.Decode(raw, &out)
	return out, err
}
func (connectionManager *Manager) ForwardPlayer(ctx context.Context, route string, req, reply proto.Message) error {
	action := clusterlink.PlayerAction(route)
	if !action.IsGame() || action == clusterlink.Attach || action == clusterlink.Disconnect {
		return access.ErrForbidden
	}
	return connectionManager.withReadyConnection(ctx, func(client *connection) error {
		if action == clusterlink.Enter || action == clusterlink.Leave {
			// 切换角色或退出后，原先的 Enter 回包不能继续证明角色已就绪。
			client.enteredRole = false
		}
		if action == clusterlink.Enter {
			enter, ok := req.(*pb.EnterReq)
			if !ok {
				return access.ErrForbidden
			}
			if err := connectionManager.directory(ctx, client, clusterlink.Admit, enter, new(pb.ListRolesRep)); err != nil {
				return err
			}
			if _, err := connectionManager.attach(ctx, client, true); err != nil {
				return err
			}
		}
		game := &client.slot.game
		if game.confirmed.binding != client.binding {
			return errors.New("player: enter a role first")
		}
		out, err := connectionManager.send(ctx, client, action, req)
		if err != nil {
			return err
		}
		if action == clusterlink.Leave {
			game.confirmed = backendTarget{}
		}
		if err = out.DecodeInto(reply); err != nil {
			return err
		}
		if entered, ok := reply.(*pb.EnterRep); action == clusterlink.Enter && ok {
			client.enteredRole = entered.GetState().GetRoleId() > 0
		}
		return nil
	})
}
