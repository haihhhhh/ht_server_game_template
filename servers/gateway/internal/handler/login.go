package handler

import (
	"context"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/gateway/internal/connection"
	"github.com/topfreegames/pitaya/v2/component"
)

type Account struct {
	component.Base
	App               *app.App
	ConnectionManager *connection.Manager
}

func (accountHandler *Account) ListZones(ctx context.Context, req *pb.ListZonesReq) (*pb.ListZonesRep, error) {
	return app.Call(ctx, accountHandler.App, "Account.ListZones", true, req, func(ctx context.Context, req *pb.ListZonesReq) (*pb.ListZonesRep, error) {
		out := new(pb.ListZonesRep)
		err := accountHandler.ConnectionManager.Directory(ctx, clusterlink.Zones, req, out)
		return out, err
	})
}

func (accountHandler *Account) Login(ctx context.Context, req *pb.LoginReq) (*pb.LoginRep, error) {
	return app.Call(ctx, accountHandler.App, "Account.Login", false, req, func(ctx context.Context, req *pb.LoginReq) (*pb.LoginRep, error) {
		id, err := accountHandler.App.Login(ctx, req.Credential)
		if err != nil {
			return nil, errcode.Public(err)
		}
		accountID, err := clusterlink.ParseAccountID(id.UserID)
		if err != nil {
			return nil, errcode.Public(err)
		}
		if accountHandler.ConnectionManager != nil {
			var authenticated context.Context
			authenticated, err = accountHandler.App.SessionContext(ctx)
			if err == nil {
				err = accountHandler.ConnectionManager.Bind(authenticated)
			}
		}
		return &pb.LoginRep{AccountId: accountID}, errcode.Public(err)
	})
}
