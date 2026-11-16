package connection

import (
	"context"
	"errors"
	"hash/fnv"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
)

func (connectionManager *Manager) directory(ctx context.Context, client *connection, action clusterlink.Action, req, reply proto.Message) error {
	if !action.IsDirectory() {
		return access.ErrForbidden
	}
	account := client.slot.account.confirmed
	requestPacket, err := clusterlink.EncodeRequest(clusterlink.Command{
		Allocation: account.assignment.AllocationID,
		Gateway:    client.binding.Gateway,
		Token:      client.binding.Token,
		Order:      client.binding.Order,
		Action:     action,
	}, req)
	if err != nil {
		return err
	}
	out := new(wrappers.BytesValue)
	if err = connectionManager.App.RPCTo(ctx, account.assignment.ServerID, "account.Accounts.Directory", out, requestPacket); err != nil {
		return err
	}
	var result clusterlink.Reply
	if err = clusterlink.Decode(out, &result); err != nil {
		return err
	}
	return result.DecodeInto(reply)
}

func (connectionManager *Manager) Directory(ctx context.Context, action clusterlink.Action, req, reply proto.Message) error {
	return connectionManager.withReadyConnection(ctx, func(client *connection) error {
		return connectionManager.directory(ctx, client, action, req, reply)
	})
}

// CreateRole 只调用创建事务，不登记玩家归属，也不加载在线 Player。
func (connectionManager *Manager) CreateRole(ctx context.Context, req *pb.CreateRoleReq, out *pb.CreateRoleRep) error {
	return connectionManager.withReadyConnection(ctx, func(client *connection) error {
		if err := connectionManager.directory(ctx, client, clusterlink.Admit, &pb.EnterReq{Zone: req.Zone}, new(pb.ListRolesRep)); err != nil {
			return err
		}
		accountID := client.binding.Identity.UserID
		assigned, found, err := connectionManager.GameRegistry.Lookup(ctx, accountID)
		if err != nil {
			return err
		}
		target := assigned.ServerID
		if !found {
			ids, discoveryErr := connectionManager.App.ServerIDs("game")
			if discoveryErr != nil {
				return discoveryErr
			}
			if len(ids) == 0 {
				return errors.New("no game server")
			}
			hash := fnv.New64a()
			_, _ = hash.Write([]byte(accountID))
			target = ids[hash.Sum64()%uint64(len(ids))]
		}
		// 超时直接交给调用方，不重试创建；下一次普通角色列表会读取已提交的数据。
		return connectionManager.App.RPCTo(ctx, target, "game.Creation.Create", out, req)
	})
}
