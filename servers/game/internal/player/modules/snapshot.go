// 【通常无需修改｜进入与整体同步】汇总各组件愿意发给客户端的数据；普通业务 Rep 不调用这里。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package modules

import pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"

// Current 收集已装配组件提供的客户端状态。加入组件后无需再登记一次同步方法。
func (components *PlayerComponents) Current() *pb.PlayerState {
	state := new(pb.PlayerState)
	for _, module := range components.Modules {
		if sender, ok := module.(interface{ FillClientState(*pb.PlayerState) }); ok {
			sender.FillClientState(state)
		}
	}
	return state
}
