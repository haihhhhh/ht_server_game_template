# 玩家业务模块

`servers/game/internal/player/modules/` 的每个业务子包拥有自己的在线组件、协议处理方法和恢复入口。同一个玩家的组件由 `PlayerComponents` 装配。进入角色后，一个玩家循环顺序处理该角色的请求与定时回调。

## 从一条经验道具请求看模块怎么写

1. 在 `inventory/restore.go` 用 `modules.Register` 登记组件、恢复函数以及前置模块 `profile`。恢复时检查角色组件并保存其 `RoleProgress` 接口。背包和角色都保存同一 `components.Config` 字段的地址。[背包装配](../servers/game/internal/player/modules/inventory/restore.go)
2. 在背包的 `action.go` 注册组件方法。分发器按请求类型找出方法，进入玩家循环后将该玩家的背包作为接收者传入。[请求登记](../servers/game/internal/player/modules/inventory/action.go)
3. `HandleUseExpItem` 直接调用自身的 `UseExpItem`。先读配置、检查库存、使用等级和经验上限，随后扣道具、调用角色组件更新经验。响应只填写道具编号、剩余数量、经验和等级。[道具规则](../servers/game/internal/player/modules/inventory/use_exp_item.go) · [完整流程图](business-request-flow.md)

```go
func init() {
    modules.RegisterAction(Feature, (*Module).HandleUseExpItem)
}
```

新增普通协议时，在协议仓声明 Req/Rep，重新生成网关转发，再参照这个登记写自己的组件方法。玩家请求统一按 Protobuf 编码大小加 128 字节估算入队占用。独立查询、扣道具、改经验等本地操作可以直接调用；需要远端调用的方法传递请求上下文以保留取消与截止时间。

## 增加一个业务子包

- 在 `modules/<业务名>/` 下定义组件，实现 `Name`、`AfterLoad`、`Enter`、`Leave`、`Snapshot`、`Accepted`。需要周期执行的组件额外实现 `Tick(time.Time)`；需要出现在整体同步中的组件实现 `FillClientState(*pb.PlayerState)`。[接口定义](../servers/game/internal/player/modules/module.go)
- 在自己的 `restore.go` 登记组件。需要数据库状态时，把模型加到项目的 `Models()`，扩展 `modules.LoadData` 和 `storage.Load`，恢复函数只使用查到的数据。新角色必须有的初始记录在角色创建事务里写入。[加载](../servers/game/internal/player/storage/load.go) · [新手初始数据](../servers/game/internal/player/storage/initialize.go)
- 增删业务子包后，从模板根目录执行 `go run ./tools/moduleimports`，或运行 Build 脚本自动生成导入文件。仅修改已有子包的方法时不需重新生成导入列表。生成文件不入库。

## 进入、定时更新和退出

`PlayerManager.load` 先选角，再通过同账号的一个 DB 任务依次查询模块数据。所有组件按依赖顺序恢复完毕后逐个执行 `AfterLoad`，检查成功才执行 `Enter`、设置就绪状态。替换连接不会重复执行 `Enter`。已有角色缺少模块行时，加载器交给组件零值和缺失标记，不重复发放五个新手道具或三次挑战机会。[恢复顺序](../servers/game/internal/player/enter.go)

- `profile` 持有角色资料、偏好和经验。改名后，保存只更新 `roles.name`；刷新角色列表读到的是已保存的名字。
- `inventory` 持有道具数量。单种最多 1,000,000 个、最多 256 种。保存只复制变化条目，数量为零时批量删除，其余批量新增或更新。[背包保存](../servers/game/internal/player/modules/inventory/inventory.go)
- `online_stats` 在进入时增加进入次数，在 Tick 和退出时按实际经过时长更新在线秒数，无需单独声明客户端接口。
- `challenge` 持有个人次数，远端 world 成功返回攻击结果后才扣除一次。[世界 Boss 示例](public-service.md)

业务日期使用传给回调的时刻或恢复时取得的 `components.BusinessNow`；统计在线多久使用 `components.OnlineDuration()`。GM 调整业务时钟不会改变 Tick 或保存的真实间隔。慢请求跨过多个 Tick 时只执行一次，不逐次补跑。[玩家调度](../servers/game/internal/player/tick.go)

周期保存和退出均通过 `Snapshot` 收集变化，没有变化就返回 `nil`。每份任务只需提供使用**独立副本**的 `Write`；变长的字符串、批量条目等通过 `ExtraBytes` 补充估算。保存入口按每个非空快照 128 字节加额外估算计数。接受入队后调用 `Accepted`；入队失败保留变化，写库失败记日志。不同模块可能只有一部分写入成功。下线先等循环停止，再执行离开回调、提交剩余变化，并等待该玩家最后一笔已提交任务，最多 5 秒。[保存入口](../servers/game/internal/player/storage/save.go)
