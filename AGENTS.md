# 模板开发约定

项目维护 `common/` 和 `servers/` 的业务代码，框架由 `go.mod` 引入。新增玩法从[开发指南](docs/development.md)和[玩家模块](docs/player-modules.md)的扩展点开始。

协议源在平级的 `ht_server_game_protocol`，表格源在 `ht_server_game_tables`。修改源文件后用仓库脚本重新生成，生成目录由 Git 忽略。修改登录、选角、普通请求或离开角色时，可用[示例客户端](docs/sample-client.md)走相应流程。
