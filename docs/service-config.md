# 服务运行配置

启动脚本的第二个参数是服务 JSON 配置，例如 `scripts\Start.bat game configs\game.json`。程序先装入默认值，再读取 `-config` 文件、应用允许的环境变量，最后校验并打开本服务所需的资源。文件最大 64 KiB；未知字段或第二个 JSON 值会导致启动失败。修改运行配置后重启进程，配置表的 `reload-tables` 单独生效。

## 哪个服务需要什么

| 服务 | 业务资源 | 服务专用 JSON 字段与默认值 |
| --- | --- | --- |
| gateway | WebSocket、在线连接、账号和玩家的归属登记查询。 | `address` 为 `127.0.0.1:3250`，`max_sessions` 为 128。下线通知完成前仍占名额，同网关顶号复用账号条目。 |
| account | MySQL、发号器、`account-owners` 归属、逻辑区目录。 | `max_accounts` 128，`account_queue` 64，`save_seconds` 10，`zones` 默认一个开放的 `default` 区。 |
| game | MySQL、发号器、Luban 表、`accounts` 玩家归属、`bosses` 公共状态归属。 | `roles_per_zone` 1，`max_players` 128，`player_queue` 64，`tick_ms` 1000，`save_seconds` 10。 |
| world | MySQL、`bosses` 归属和 Boss 内存状态。 | `max_scopes` 128，`queue_capacity` 64，`save_seconds` 10。 |

四个服务均连接 NATS 和 etcd 并使用环境业务时钟。`db_queues` 默认为 4，打开数据库时接受 1–8。只有 game 打开 `tables` 指定的表包，默认目录为 `tables/exported`。网关的 `accounts` 登记按账号 ID 指向 game，`account-owners` 登记指向在线账号所在的 account。样例见 [configs](../configs/)。

`zones` 中每项含 `id`、`name` 和 `open`。共 1–100 项，ID 唯一，ID 和名称分别最多 64、128 字节。各 account 实例使用同一份区服配置。玩家 Tick 按进入后经过的时间触发；慢请求跨过多个周期时合并为一次。周期保存从每次尝试结束起重新计时。详见[玩家模块](player-modules.md)。

## 环境变量

| 变量 | 作用 |
| --- | --- |
| `HT_NATS`、`HT_ETCD` | 覆盖 JSON 中的 `nats`、`etcd`。地址格式、认证和网络隔离见 [NATS 配置](nats.md)。 |
| `HT_NATS_USER`、`HT_NATS_PASSWORD` | 各个服务进程都需要的 NATS 凭据。 |
| `HT_MYSQL_DSN` | account、game、world 的数据库连接串，必须带库名。 |
| `HT_DEV_TICKET` | account 和示例客户端使用相同的开发票据。 |
| `HT_INSTANCE`、`HT_LOG_LEVEL` | 分别覆盖实例日志标识和业务日志等级。`instance` 留空时生成本次进程的标识。实例日志标识与 Pitaya 发给本次运行的 `server_id` 各有用途。 |
| `HT_DIAGNOSTICS_ADDR`、`HT_DIAGNOSTICS_TOKEN`、`HT_PPROF`、`HT_TRACE` | 控制回环诊断端口、访问凭据、性能采样和链路追踪，见[日志与诊断](observability.md)。 |

## 发号节点 id_node

account、game 须在 JSON 中显式设置 `id_node`，范围 0–1023。示例分别使用 1 和 2。新增发号进程需选择独立编号，即使服务类型或发现前缀不同，只要共享业务 ID 空间，就应使用同一套 etcd 节点登记。进程启动时独占 `/ht/idgen/nodes/<编号>/owner`，此键无 TTL。重号时启动失败。创建账号和角色在本地生成正的 `int64`，Pitaya 会话和 etcd 业务归属的键使用账号 ID 的十进制文本。

正常停服时发号器在同一 etcd 事务内写入 `last-id` 并释放自身的 `owner`。异常退出后先确认旧进程不会再运行，保留 `last-id`。复用编号前须确认机器时钟超过旧进程可能发号的最后时刻，读取 owner 与修订号，以 etcd 事务比对修订号后仅删除该 owner。无法确认时，为新进程分配从未使用的编号并保留旧占用。
