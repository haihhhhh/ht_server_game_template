package handler

import (
	"context"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/account"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/directory"
	"github.com/topfreegames/pitaya/v2/component"
)

// AccountSessionRPC 处理在线账号绑定、目录查询和断线通知。
type AccountSessionRPC struct {
	component.Base
	App              *app.App
	AccountManager   *account.AccountManager
	AccountRegistry  *placement.Registry
	DirectoryService *directory.Service
}

func (accountsHandler *AccountSessionRPC) Attach(ctx context.Context, q *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.CallRemote(ctx, accountsHandler.App, "Accounts.Attach", false, q, func(ctx context.Context, q *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		command, binding, err := readAccountCommand(ctx, q)
		if err != nil {
			return nil, err
		}
		assignment := placement.Assignment{
			Scope:        binding.Identity.UserID,
			ServerID:     accountsHandler.App.ServerID(),
			AllocationID: command.Allocation,
		}
		reply, err := accountsHandler.AccountManager.Attach(ctx, assignment, binding, command.Claim)
		if err != nil {
			return nil, errcode.Public(err)
		}
		return clusterlink.Encode(reply)
	})
}

func (accountsHandler *AccountSessionRPC) Closed(ctx context.Context, q *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.CallRemote(ctx, accountsHandler.App, "Accounts.Closed", true, q, func(ctx context.Context, q *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		command, binding, err := readAccountCommand(ctx, q)
		if err != nil {
			return nil, err
		}
		if command.Claim || command.Action != clusterlink.Disconnect {
			return nil, access.ErrForbidden
		}
		err = accountsHandler.AccountManager.Disconnect(ctx, command.Allocation, binding)
		return &wrappers.BytesValue{}, err
	})
}

func (accountsHandler *AccountSessionRPC) decode(ctx context.Context, q *wrappers.BytesValue) (placement.Assignment, clusterlink.Binding, error) {
	command, binding, err := readAccountCommand(ctx, q)
	if err != nil {
		return placement.Assignment{}, binding, err
	}
	if command.Claim {
		return placement.Assignment{}, binding, access.ErrForbidden
	}
	assignment, found, err := accountsHandler.AccountRegistry.Lookup(ctx, binding.Identity.UserID)
	if err != nil {
		return assignment, binding, err
	}
	if !found || assignment.ServerID != accountsHandler.App.ServerID() || assignment.AllocationID != command.Allocation {
		return assignment, binding, access.ErrForbidden
	}
	return assignment, binding, nil
}

func readAccountCommand(ctx context.Context, q *wrappers.BytesValue) (clusterlink.Command, clusterlink.Binding, error) {
	var command clusterlink.Command
	id, err := access.FromContext(ctx)
	if err != nil {
		return command, clusterlink.Binding{}, err
	}
	if clusterlink.Decode(q, &command) != nil || command.Gateway == "" || len(command.Gateway) > 256 || command.Allocation == "" || len(command.Allocation) > 128 || len(command.Token) < 16 || len(command.Token) > 128 {
		return command, clusterlink.Binding{}, access.ErrForbidden
	}
	return command, clusterlink.Binding{
		Gateway:  command.Gateway,
		Identity: id,
		Token:    command.Token,
		Order:    command.Order,
	}, nil
}
