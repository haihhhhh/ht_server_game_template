package account

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/haihhhhh/ht_server_frame/access"
)

// DevelopmentVerifier 只供显式开启的本机开发使用；正式项目替换为渠道验证。
func DevelopmentVerifier(ticket string) (Verifier, error) {
	if len(ticket) < 16 || len(ticket) > 128 {
		return nil, errors.New("HT_DEV_TICKET must contain 16 to 128 bytes")
	}
	return func(ctx context.Context, credential string) (Identity, error) {
		if err := ctx.Err(); err != nil {
			return Identity{}, err
		}
		if subtle.ConstantTimeCompare([]byte(ticket), []byte(credential)) != 1 {
			return Identity{}, access.ErrUnauthenticated
		}
		return Identity{
			Channel: "development",
			Subject: "local-player",
		}, nil
	}, nil
}
