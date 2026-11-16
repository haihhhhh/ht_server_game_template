// worldlink 是 game 与公共服务的项目内部约定，不暴露给客户端。
package worldlink

import "encoding/base64"

type Attack struct {
	Scope      string
	Allocation string
	Damage     int64
}
type Result struct{ HP int64 }

// 示例每个逻辑区服一个 Boss；以后可换成活动 ID，不影响玩家归属。
func Scope(zone string) string {
	return "boss:" + base64.RawURLEncoding.EncodeToString([]byte(zone))
}
