package handler

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/account"
)

func (accountsHandler *AccountSessionRPC) check(ctx context.Context, accountID int64, binding clusterlink.Binding) error {
	_, err := accountsHandler.AccountManager.Call(ctx, accountID, 256, func(_ context.Context, onlineAccount *account.Account) (any, error) {
		return nil, onlineAccount.Check(binding)
	})
	return err
}

// Directory 查询等待在账号循环外；返回前重新核对连接，丢弃旧登录的结果。
func (accountsHandler *AccountSessionRPC) Directory(ctx context.Context, requestPacket *wrappers.BytesValue) (*wrappers.BytesValue, error) {
	return app.CallRemote(ctx, accountsHandler.App, "Accounts.Directory", false, requestPacket, func(ctx context.Context, requestPacket *wrappers.BytesValue) (*wrappers.BytesValue, error) {
		_, binding, err := accountsHandler.decode(ctx, requestPacket)
		if err != nil {
			return nil, err
		}
		accountID, err := clusterlink.ParseAccountID(binding.Identity.UserID)
		if err != nil {
			return nil, err
		}
		if err = accountsHandler.check(ctx, accountID, binding); err != nil {
			return nil, err
		}
		var command clusterlink.Command
		if err = clusterlink.Decode(requestPacket, &command); err != nil {
			return nil, err
		}
		request, err := command.Request()
		if err != nil {
			return nil, err
		}
		var result proto.Message
		if !command.Action.IsDirectory() {
			return nil, access.ErrForbidden
		}
		switch command.Action {
		case clusterlink.Check:
			// 登录末尾只确认绑定，上面已在账号循环内核对，可以直接回复。
			return clusterlink.EncodeReply(&pb.ListRolesRep{})
		case clusterlink.Zones:
			out := &pb.ListZonesRep{}
			for _, zone := range accountsHandler.DirectoryService.Zones {
				out.Zones = append(out.Zones, &pb.ZoneInfo{
					Id:   zone.ID,
					Name: zone.Name,
					Open: zone.Open,
				})
			}
			result = out
		case clusterlink.Roles, clusterlink.Admit:
			req := request.(*pb.EnterReq)
			if err = accountsHandler.DirectoryService.CheckZone(req.Zone, command.Action == clusterlink.Admit); err != nil {
				return nil, err
			}
			out := &pb.ListRolesRep{}
			if command.Action == clusterlink.Roles || req.RoleId != 0 {
				values, queryErr := accountsHandler.DirectoryService.Roles(ctx, accountID, req.Zone, req.RoleId)
				if queryErr != nil {
					return nil, queryErr
				}
				if command.Action == clusterlink.Admit && (len(values) != 1 || values[0].Status != model.Normal) {
					return nil, errcode.RoleForbidden()
				}
				for _, role := range values {
					out.Roles = append(out.Roles, &pb.RoleInfo{
						Id:     role.ID,
						Zone:   role.Zone,
						Name:   role.Name,
						Status: role.Status,
					})
				}
			}
			result = out
		default:
			return nil, access.ErrForbidden
		}
		if err = accountsHandler.check(ctx, accountID, binding); err != nil {
			return nil, err
		}
		return clusterlink.EncodeReply(result)
	})
}
