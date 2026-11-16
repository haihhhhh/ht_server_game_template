package connection

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
)

// confirmed 是后端最后确认的绑定。换绑请求可能已执行但尚未收到回包时，还要保留 pending。
type backendLink struct {
	confirmed backendTarget
	pending   backendTarget
}

type backendTarget struct {
	assignment placement.Assignment
	binding    clusterlink.Binding
}

const (
	gameClosedRoute    = "game.Cluster.Closed"
	accountClosedRoute = "account.Accounts.Closed"
)

type closeBackend func(context.Context, string, backendTarget) error

func (target *backendTarget) close(ctx context.Context, route string, notify closeBackend) error {
	if target.assignment.AllocationID == "" {
		return nil
	}
	if err := notify(ctx, route, *target); err != nil {
		return fmt.Errorf("%s: %w", route, err)
	}
	*target = backendTarget{}
	return nil
}

func (slot *accountEntry) closePending(ctx context.Context, notify closeBackend) error {
	return errors.Join(
		slot.game.pending.close(ctx, gameClosedRoute, notify),
		slot.account.pending.close(ctx, accountClosedRoute, notify),
	)
}

func (slot *accountEntry) notifyClosed(ctx context.Context, notify closeBackend) error {
	return errors.Join(
		slot.closePending(ctx, notify),
		slot.game.confirmed.close(ctx, gameClosedRoute, notify),
		slot.account.confirmed.close(ctx, accountClosedRoute, notify),
	)
}

type sessionCloseRPC func(context.Context, access.Identity, string, string, proto.Message, proto.Message) error

func notifyBackend(send sessionCloseRPC) closeBackend {
	return func(ctx context.Context, route string, target backendTarget) error {
		requestPacket, err := clusterlink.Encode(clusterlink.Command{
			Allocation: target.assignment.AllocationID,
			Gateway:    target.binding.Gateway,
			Action:     clusterlink.Disconnect,
			Token:      target.binding.Token,
			Order:      target.binding.Order,
		})
		if err != nil {
			return err
		}
		callContext, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return send(callContext, target.binding.Identity, target.assignment.ServerID, route, new(wrappers.BytesValue), requestPacket)
	}
}
