package errcode

import (
	"errors"

	perror "github.com/topfreegames/pitaya/v2/errors"
)

const (
	CodePlayerBusy  = "GAME-PLAYER-BUSY"
	CodeAccountBusy = "GAME-ACCOUNT-BUSY"
)

var (
	ErrNoChallenges     = errors.New("no challenges left")
	ErrBossDefeated     = errors.New("boss defeated")
	ErrWorldUnavailable = errors.New("world unavailable")
)
var ErrPlayerBusy = errors.New("player: loading or leaving")
var ErrAccountBusy = errors.New("account: loading or leaving")

// RoleForbidden 让 account 和 game 在拒绝选角时返回同一错误，避免透露其他账号的角色是否存在。
func RoleForbidden() error {
	const code = "GAME-ROLE-FORBIDDEN"
	return perror.NewError(errors.New(code), code)
}

// Public 只公开项目确认的业务错误，数据库错误继续交给框架脱敏。
func Public(err error) error {
	code := ""
	switch {
	case errors.Is(err, ErrNoChallenges):
		code = "GAME-NO-CHALLENGES"
	case errors.Is(err, ErrBossDefeated):
		code = "GAME-BOSS-DEFEATED"
	case errors.Is(err, ErrWorldUnavailable):
		code = "GAME-WORLD-UNAVAILABLE"
	case errors.Is(err, ErrPlayerBusy):
		code = CodePlayerBusy
	case errors.Is(err, ErrAccountBusy):
		code = CodeAccountBusy

	}
	if code != "" {
		return perror.NewError(errors.New(code), code)
	}
	return err
}
