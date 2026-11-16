package connection

import (
	"context"
	"errors"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/observability"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
)

func (connectionManager *Manager) attachAccount(ctx context.Context, client *connection) (clusterlink.Binding, error) {
	accountID := client.binding.Identity.UserID
	assignment, found, err := connectionManager.AccountRegistry.Lookup(ctx, accountID)
	if err != nil {
		return clusterlink.Binding{}, err
	}
	if !found {
		assignment, err = clusterlink.SelectCandidate(connectionManager.App, "account", accountID)
		if err != nil {
			return clusterlink.Binding{}, err
		}
	}
	account := &client.slot.account
	assignment, reply, err := attachOwner(assignment, !found, func(target placement.Assignment, claim bool) (clusterlink.Reply, error) {
		candidate := backendTarget{assignment: target, binding: client.binding}
		return connectionManager.sendAttach(ctx, account, candidate, "account.Accounts.Attach", clusterlink.Command{Claim: claim})
	})
	if err != nil {
		return clusterlink.Binding{}, err
	}
	if !reply.Current.Order.Valid() || !reply.Current.SameConnection(client.binding) || (assignment.Revision != 0 && reply.Current.Order.OwnerRevision != assignment.Revision) {
		return clusterlink.Binding{}, errors.New("account: invalid login binding")
	}
	client.binding.Order = reply.Current.Order
	assignment.Revision = reply.Current.Order.OwnerRevision
	account.confirmed = backendTarget{assignment: assignment, binding: reply.Current}
	account.pending = backendTarget{}
	return reply.Previous, nil
}

func (connectionManager *Manager) closePrevious(ctx context.Context, old clusterlink.Binding) error {
	if old.Gateway == "" || old.Gateway == connectionManager.App.ServerID() {
		return nil
	}
	requestPacket, err := clusterlink.Encode(old)
	if err != nil {
		return err
	}
	if err = connectionManager.App.RPCTo(ctx, old.Gateway, "gateway.Connections.Replace", new(wrappers.BytesValue), requestPacket); err != nil {
		observability.Logger(ctx).Warn("旧网关连接关闭未确认，新绑定保持生效", "component", "connection", "stage", "replace")
	}
	return nil
}
