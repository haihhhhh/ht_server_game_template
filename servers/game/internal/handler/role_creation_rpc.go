package handler

import (
	"context"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/role"
	"github.com/topfreegames/pitaya/v2/component"
)

// RoleCreationRPC 接收创建角色的内部请求，不加载在线玩家。
type RoleCreationRPC struct {
	component.Base
	App         *app.App
	RoleService *role.Service
}

func (creationHandler *RoleCreationRPC) Create(ctx context.Context, req *pb.CreateRoleReq) (*pb.CreateRoleRep, error) {
	return app.CallRemote(ctx, creationHandler.App, "Creation.Create", false, req, func(ctx context.Context, req *pb.CreateRoleReq) (*pb.CreateRoleRep, error) {
		id, err := access.FromContext(ctx)
		if err != nil {
			return nil, err
		}
		accountID, err := clusterlink.ParseAccountID(id.UserID)
		if err != nil {
			return nil, err
		}
		v, err := creationHandler.RoleService.Create(ctx, accountID, req.Zone, req.Name)
		if err != nil {
			return nil, publicError(err)
		}
		return &pb.CreateRoleRep{
			Role: &pb.RoleInfo{
				Id:     v.ID,
				Zone:   v.Zone,
				Name:   v.Name,
				Status: v.Status,
			},
		}, nil
	})
}
