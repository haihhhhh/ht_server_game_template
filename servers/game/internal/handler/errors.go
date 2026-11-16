package handler

import (
	"errors"

	"github.com/haihhhhh/ht_server_frame/execution"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules/inventory"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules/profile"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/role"
	perror "github.com/topfreegames/pitaya/v2/errors"
)

func publicError(err error) error {
	code := ""
	switch {
	case errors.Is(err, inventory.ErrItem):
		code = "GAME-INVALID-ITEM"
	case errors.Is(err, inventory.ErrInsufficient):
		code = "GAME-NOT-ENOUGH-ITEMS"
	case errors.Is(err, inventory.ErrLimit):
		code = "GAME-INVENTORY-LIMIT"
	case errors.Is(err, inventory.ErrLevel):
		code = "GAME-LEVEL-REQUIRED"
	case errors.Is(err, profile.ErrExperience):
		code = "GAME-EXPERIENCE-LIMIT"
	case errors.Is(err, profile.ErrPreferencesTooLong):
		return execution.ErrInvalid
	case errors.Is(err, role.ErrForbidden):
		return errcode.RoleForbidden()
	case errors.Is(err, role.ErrLimit):
		code = "GAME-ROLE-LIMIT"
	case errors.Is(err, role.ErrChoose):
		code = "GAME-CHOOSE-ROLE"
	}
	if code != "" {
		return perror.NewError(errors.New(code), code)
	}
	return errcode.Public(err)
}
