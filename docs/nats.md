# 配置 NATS 和集群地址

gateway、account、game 和 world 都通过 NATS 通信。同一环境的四个用户放入同一个 NATS account（消息命名空间），可以相互调用。业务账号、角色和当前连接由服务处理。环境内任一 NATS 凭据都具有该消息空间的访问能力，按服务分别注入和保管密码。

## 本机启动

1. 在启动 NATS 的终端设置 `HT_NATS_GATEWAY_PASSWORD`、`HT_NATS_ACCOUNT_PASSWORD`、`HT_NATS_GAME_PASSWORD`、`HT_NATS_WORLD_PASSWORD`，为各用户提供密码。
2. 在模板根目录运行 `nats-server -c ops/nats/server.conf`。该[配置文件](../ops/nats/server.conf)监听 `127.0.0.1:14222`，四个用户都在 `GAME_DEV` 消息空间。
3. 启动每个游戏服务前，在该进程的环境中设置 `HT_NATS_USER` 和 `HT_NATS_PASSWORD`。例如 game 使用用户名 `game` 和上面为 game 设置的密码。模板将二者编码进 Pitaya RPC 客户端和服务端的连接配置，JSON 和日志中不要写密码。

## 多地址和跨机部署

`nats` 或 `HT_NATS` 接受 `nats://IP:端口`、`tls://IP:端口`，最多 16 个，逗号分隔。只填写地址，URL 内的用户名、密码、路径和查询参数会被拒绝。`tls://` 使用 NATS 客户端的系统信任证书，证书须适用于所填 IP。

`private_network` 默认为 `false`，NATS、etcd 和前端监听使用回环 IP。跨机时在可信内网显式启用 `private_network`，NATS 和 etcd 地址可填写私有或回环 IP。框架接受的 etcd 格式为 `http://IP:端口`。不能在这些位置用 DNS 名代替 IP。对外的网关监听和基础服务隔离由部署者配置。

多个 NATS 地址只向客户端提供可连接的节点。部署 NATS 集群时，另行配置节点间的 routes、认证和一致的账号信息。不同环境使用隔离的消息空间；etcd 的 `prefix` 只区分发现及相应业务键，不能隔离 NATS 消息。
