package clusterlink

import pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"

// Action 是内部控制指令，不是客户端消息编号；值保持兼容现有 RPC 数据。
type Action string

const (
	Attach      Action = "attach"
	Disconnect  Action = "disconnect"
	Enter       Action = "enter"
	Leave       Action = "leave"
	Current     Action = "current"
	Preferences Action = "preferences"
	AttackBoss  Action = "attack-boss"
	UseExpItem  Action = "use-exp-item"
	Check       Action = "check"
	Zones       Action = "zones"
	Roles       Action = "roles"
	Admit       Action = "admit"
)

func (a Action) IsGame() bool {
	switch a {
	case Attach, Disconnect, Enter, Leave, Current, Preferences, AttackBoss, UseExpItem:
		return true
	default:
		return pb.NewPlayerForwardRequest(string(a)) != nil
	}
}

func (a Action) IsDirectory() bool {
	switch a {
	case Check, Zones, Roles, Admit:
		return true
	default:
		return false
	}
}

// PlayerAction 保留已有内部指令的线上取值；新增接口直接使用协议声明的路由。
func PlayerAction(route string) Action {
	switch route {
	case "Player.Enter":
		return Enter
	case "Player.Leave":
		return Leave
	case "Player.Current":
		return Current
	case "Player.SetPreferences":
		return Preferences
	case "Player.AttackBoss":
		return AttackBoss
	case "Player.UseExpItem":
		return UseExpItem
	default:
		return Action(route)
	}
}
