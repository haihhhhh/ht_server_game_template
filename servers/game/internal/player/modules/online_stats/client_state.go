// TODO(业务扩展)【进入与整体同步】：在 FillClientState 填写本组件允许客户端看到的字段；修改协议后重新生成，不直接返回数据库模型。
package onlinestats

import pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"

func (onlineStatsModule *Module) FillClientState(state *pb.PlayerState) {
	// 保留协议字段名；此处统计角色进入次数，不是账号认证次数。
	state.LoginCount = onlineStatsModule.data.LoginCount
	state.OnlineSeconds = onlineStatsModule.data.OnlineSeconds
}
