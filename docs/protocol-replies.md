# 请求与响应

在配套协议仓的 `game.proto` 中用 `service/rpc` 配对请求和响应，将消息加入目录后运行 `scripts/Generate-Protocol.bat` 或 `sh scripts/Generate-Protocol.sh`。客户端使用相同定义生成自己的类型和映射。网络层用请求序号关联调用，用消息编号识别类型。业务方法返回具体的 Protobuf 消息即可。

| 接口 | 请求 | 成功响应 |
| --- | --- | --- |
| Account.Login | LoginReq | LoginRep，内部账号编号 |
| Account.ListZones | ListZonesReq | ListZonesRep，逻辑区目录 |
| Roles.List | ListRolesReq | ListRolesRep，指定区的角色列表 |
| Roles.Create | CreateRoleReq | CreateRoleRep，本次创建的角色 |
| Player.Enter | EnterReq | EnterRep，角色的完整客户端状态 |
| Player.Current | CurrentReq | CurrentRep，显式请求的完整客户端状态 |
| Player.SetPreferences | SetPreferencesReq | SetPreferencesRep，修改后的设置 |
| Player.UseExpItem | UseExpItemReq | UseExpItemRep，道具编号、剩余数量、经验和等级 |
| Player.AttackBoss | AttackBossReq | AttackBossRep，Boss 剩余血量与个人剩余次数 |
| Player.Leave | LeaveReq | LeaveRep，退出流程的最后保存确认标记 |

普通玩法只返回与本次操作相关的当前值。例如使用道具后，客户端用 `remaining_count` 替换本地数量，零表示移除。进入角色或明确调用 `Current` 时再汇总整个 `PlayerState`。请求失败走 Pitaya 错误通道，客户端从错误包读取错误码。[示例客户端](../cmd/client/main.go) · [道具请求流程图](business-request-flow.md)

账号和角色编号均为 Protobuf `int64`。客户端从列表取出角色编号并原样传入 `EnterReq.role_id`，值为 0 表示自动选角。JavaScript 客户端使用所选生成器支持的 64 位表示，避免用可能丢精度的普通 Number 保存大 ID。调整消息编号或字段类型时，应同步更新双方协议。连接不会协商业务编号版本。

新增普通玩家接口时，在 `Player` 服务声明 RPC，重新生成网关转发入口，在对应玩家组件登记处理方法。成功响应反映当次业务处理结果；经验、道具等在线状态随后定期保存。[登记组件方法](player-modules.md)
