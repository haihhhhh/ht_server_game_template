package handler

import (
	"context"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/gateway/internal/connection"
	"github.com/topfreegames/pitaya/v2/component"
)

// Roles 客户端角色列表和创建入口；实际查询或创建交给后端服务。
type Roles struct {
	component.Base
	App               *app.App
	ConnectionManager *connection.Manager
}

func (rolesHandler *Roles) List(ctx context.Context, req *pb.ListRolesReq) (*pb.ListRolesRep, error) {
	return app.Call(ctx, rolesHandler.App, "Roles.List", true, req, func(ctx context.Context, req *pb.ListRolesReq) (*pb.ListRolesRep, error) {
		out := new(pb.ListRolesRep)
		err := rolesHandler.ConnectionManager.Directory(ctx, clusterlink.Roles, &pb.EnterReq{Zone: req.Zone}, out)
		return out, err
	})
}

func (rolesHandler *Roles) Create(ctx context.Context, req *pb.CreateRoleReq) (*pb.CreateRoleRep, error) {
	return app.Call(ctx, rolesHandler.App, "Roles.Create", true, req, func(ctx context.Context, req *pb.CreateRoleReq) (*pb.CreateRoleRep, error) {
		out := new(pb.CreateRoleRep)
		err := rolesHandler.ConnectionManager.CreateRole(ctx, req, out)
		return out, err
	})
}
