# ht_server_game_template

Go 游戏服务端项目模板。连接由 `gateway` 管理，`account` 验证凭据并维护在线账号，`game` 加载角色并在玩家自己的循环中执行业务，`world` 演示独立的世界 Boss 服务。进程通过 Pitaya 的 NATS RPC 通信。

## 运行示例

1. 将模板、`ht_server_frame`、`ht_server_game_protocol` 和 `ht_server_game_tables` 放在同一父目录。模板的 `go.mod` 直接引用平级框架。使用配套的 `game.proto`，其中账号 ID 和角色 ID 为 `int64`，客户端也用同一份协议生成代码。
2. 生成协议和配置表，构建五个入口。准备 NATS、etcd、MySQL，在独立终端启动账号服、游戏服和网关。运行世界 Boss 示例时再启动 `world`。[从源码启动](docs/getting-started.md)
3. 使用随模板提供的客户端查看区服与角色、进入游戏、使用经验道具或挑战 Boss。示例开发票据对应一个固定账号。[示例客户端](docs/sample-client.md)

## 开发入口

| 你要做的事 | 从这里开始 |
| --- | --- |
| 创建自己的服务端、协议和表格仓库 | [从模板创建项目](docs/create-project.md) |
| 修改登录、选角或同账号顶号 | [账号与角色](docs/accounts-and-roles.md) · [网关与节点归属](docs/cluster.md) |
| 增加玩家协议和玩法 | [开发指南](docs/development.md) · [玩家模块](docs/player-modules.md) · [一次道具请求的调用顺序](docs/business-request-flow.md) |
| 读配置表、保存业务数据 | [数据模型和配置表](docs/data-models.md) · [数据库服务](docs/database.md) |
| 拆出多人共享的业务 | [状态范围](docs/state-scopes.md) · [公共 Boss 示例](docs/public-service.md) |
| 配置、查看日志和维护服务 | [服务配置](docs/service-config.md) · [日志与诊断](docs/observability.md) · [运行手册](docs/operations.md) |

账号和角色创建完成后才返回成功。玩家在线修改先作用于内存，定期按变化提交保存。节点故障时，最近的修改可能丢失；恢复有归属的数据前先确认旧节点已停止写入。

本项目采用 [MIT 许可证](LICENSE)。
