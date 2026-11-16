package profile

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
)

func (profileModule *Module) RestoreRole(role model.Role) {
	profileModule.role = role
	profileModule.acceptedName = role.Name
}
func (profileModule *Module) Role() model.Role {
	return profileModule.role
}

// SetName 由玩家循环调用；收费、重名等项目规则应在调用前检查。
func (profileModule *Module) SetName(name string) error {
	if name == "" || len(name) > 64 || !utf8.ValidString(name) || strings.TrimSpace(name) != name || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return errors.New("invalid role name")
	}
	profileModule.role.Name = name
	return nil
}

func (profileModule *Module) ClientState() *pb.PlayerState {
	state := new(pb.PlayerState)
	profileModule.FillClientState(state)
	return state
}

func (profileModule *Module) FillClientState(state *pb.PlayerState) {
	state.RoleId = profileModule.role.ID
	state.Name = profileModule.role.Name
	state.Preferences = profileModule.data.Preferences
	state.Experience = profileModule.data.Experience
	state.Level = profileModule.Level()
}
