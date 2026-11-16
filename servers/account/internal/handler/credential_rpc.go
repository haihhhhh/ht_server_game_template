package handler

import (
	"context"
	"errors"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/account"
	"github.com/topfreegames/pitaya/v2/component"
	perror "github.com/topfreegames/pitaya/v2/errors"
)

// CredentialRPC 在玩家绑定连接之前验证账号凭据。
type CredentialRPC struct {
	component.Base
	App            *app.App
	AccountService *account.Service
}

// Authenticate 仅注册为内部 RPC；服务认证由 NATS 完成，此时尚无玩家登录身份。
func (credentialsHandler *CredentialRPC) Authenticate(ctx context.Context, req *pb.LoginReq) (*pb.LoginRep, error) {
	return app.Call(ctx, credentialsHandler.App, "Credentials.Authenticate", false, req, func(ctx context.Context, req *pb.LoginReq) (*pb.LoginRep, error) {
		if req == nil || len(req.Credential) == 0 || len(req.Credential) > 128 {
			return nil, access.ErrUnauthenticated
		}
		id, err := credentialsHandler.AccountService.Login(ctx, req.Credential)
		switch {
		case errors.Is(err, account.ErrBanned):
			err = perror.NewError(errors.New("GAME-ACCOUNT-BANNED"), "GAME-ACCOUNT-BANNED")
		case errors.Is(err, account.ErrRegistrationDenied):
			err = perror.NewError(errors.New("GAME-REGISTRATION-DENIED"), "GAME-REGISTRATION-DENIED")
		case errors.Is(err, account.ErrInvalidIdentity):
			err = access.ErrUnauthenticated
		}
		return &pb.LoginRep{AccountId: id}, err
	})
}
