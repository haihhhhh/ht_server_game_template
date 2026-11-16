package gateway

import (
	"context"
	"errors"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/errorcode"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/servers/gateway/internal/connection"
	perror "github.com/topfreegames/pitaya/v2/errors"
)

// authenticationError 只允许已约定的账号业务码通过，丢弃远端消息及附带信息。
func authenticationError(err error) error {
	switch code := errorcode.Of(err); code {
	case "GAME-ACCOUNT-BANNED", "GAME-REGISTRATION-DENIED", errorcode.Overloaded:
		return perror.NewError(errors.New(code), code)
	default:
		return nil
	}
}

func configureAuthentication(gatewayApp *app.App, connectionManager *connection.Manager, maxSessions int) error {
	verifyCredential := func(ctx context.Context, credential string) (string, error) {
		response := &pb.LoginRep{}
		rpcErr := gatewayApp.RPC(ctx, "account.Credentials.Authenticate", response, &pb.LoginReq{Credential: credential})
		if rpcErr != nil {
			return "", rpcErr
		}
		if response.AccountId <= 0 {
			return "", access.ErrUnauthenticated
		}
		return clusterlink.AccountUID(response.AccountId), nil
	}
	return gatewayApp.ConfigureAuthentication(
		access.Options{
			Scope:              "local",
			Policy:             access.ReplaceOld,
			MaxSessions:        maxSessions,
			MaxCredentialBytes: 128,
			Verify:             verifyCredential,
			PublicError:        authenticationError,
			BeforeBind:         connectionManager.ReserveConnection,
			OnDisconnected:     connectionManager.OnDisconnected,
			DisconnectWorkers:  connection.DisconnectQueueCount,
		},
	)
}
