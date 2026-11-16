# 进程内数据库服务

account、game、world 各自打开 MySQL 连接池和一组数据库任务队列。gateway 不连库。账号和角色主键为有符号 64 位整数，由项目发号器创建。配置 `HT_MYSQL_DSN` 指向已创建的数据库，数据库账号需有建表和迁移权限。

## 启动建表

服务开始接收请求前，`database.Open` 在同一条数据库连接上取得 MySQL 命名锁，再按 `Models()` 清单逐个执行 GORM `AutoMigrate`。多个进程使用同一数据库时串行迁移。获取锁最多等 5 秒；未取得锁或任一模型迁移失败都会使启动返回错误。此前已成功执行的 MySQL DDL 可能已生效。增加模型时在共享清单登记一次；需要转换存量业务数据的改动应另行处理。[模型清单](../common/model/database/models.go) · [启动迁移](../infrastructure/database/schema.go)

## 排队与读取结果

| 入口 | 适用场景 | 完成的含义 |
| --- | --- | --- |
| `database.Query(ctx, service, key, size, fn)` | 登录查号、选角、加载业务数据，或要立刻使用结果的写操作 | 调用方等待结果或上下文结束。等待超时不代表正在执行的写操作一定没有生效。 |
| `service.Submit(ctx, key, size, fn)` | 组件将变化副本交给后台保存 | 返回时可判断是否入队，返回的通道交付最终执行错误。已提交的后台任务有自己的执行期限。 |
| `database.WaitForSave(ctx, lastSave, submitErr)` | 退出时等待最后一笔保存 | 返回最后一笔的结果和本次提交错误；较早批次可能已经失败。 |

相同键在**同一进程**总会进入同一条队列。角色读写和玩家组件保存使用 `database.AccountKey(accountID)`；注册前使用渠道、用户标识编码的键；Boss 使用其范围标识。回调只使用传入的 DB 句柄和独立数据，不再排一个任务并等待同一队列。跨进程的并发创建还要依靠唯一索引、锁或事务。[任务提交与等待](../infrastructure/database/task.go)

`db_queues` 默认为 4，可配 1–8。每条队列最多接纳 256 个未完成任务，估算总量上限 4 MiB；执行超时 3 秒。连接池最多 8 个连接。超限会拒绝入队。停止时先关闭所有队列的接单入口，排空后关闭连接池。`Service.Close` 最多等待 5 秒；如果超时，后台在进程仍运行时继续排空。[数据库服务](../infrastructure/database/service.go)

玩家模块定期保存将同一玩家的变化合成一个任务，各模块依次写入，各模块之间没有共同事务。异步任务失败会记日志，不会自动重放。单个节点的队列也不能阻止其他节点的旧写入。[组件保存](../servers/game/internal/player/storage/save.go)
