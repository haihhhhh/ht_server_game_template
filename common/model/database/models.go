// Package database 定义保存到数据库的记录，不包含协议、配置表或在线组件。
package database

// Models 是项目统一的建表清单；所有连接项目库的服务启动时都初始化这些表。
// TODO(业务扩展)：新增数据库模型在此登记一次，与哪个组件使用它无关。
func Models() []any {
	return []any{
		&Account{},
		&Role{},
		&PlayerProfile{},
		&PlayerItem{},
		&PlayerOnlineStats{},
		&PlayerChallenge{},
		&WorldBoss{},
	}
}
