package account

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrBanned = errors.New("account: login forbidden")
var ErrInvalidIdentity = errors.New("account: invalid identity")
var ErrRegistrationDenied = errors.New("account: registration denied")

// Identity 只能由可信渠道验证器生成，不能从客户端请求直接反序列化。
type Identity struct{ Channel, Subject string }
type Verifier func(context.Context, string) (Identity, error)

func valid(id Identity) bool {
	return validPart(id.Channel, 32) && validPart(id.Subject, 191)
}

func validPart(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	return strings.IndexFunc(value, unicode.IsControl) < 0
}
