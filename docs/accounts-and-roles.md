# 账号、角色与进入游戏

账号把渠道用户映射为项目自己的 `int64` 编号。角色是属于某个账号、某个逻辑区的长期档案。玩家进入角色后，game 才创建内存中的 `Player`，加载角色资料、背包等组件。

## 登录与注册

1. 客户端发送 `Account.Login`。gateway 将凭据交给内部 `account.Credentials.Authenticate`。验证器返回可信的渠道和用户标识。模板的 `DevelopmentVerifier` 只接受 `HT_DEV_TICKET`，映射到一个固定开发账号。
2. account 按渠道和用户标识查询账号。未命中时才进入创建流程，再查一次、执行可选的 `BeforeCreate` 并使用发号器创建账号。数据库的 `(channel, subject)` 唯一索引处理并发注册。同一身份再次登录会取得同一个账号编号。
3. 仅 `normal` 状态允许登录。gateway 取得账号编号后绑定当前连接，与在线账号的负责人关联；已有 game 归属时还要更新 game 的绑定。登录不会创建角色。[在线账号管理](online-accounts.md)

正式接入时，在 account 的服务装配里替换验证器。渠道名和用户标识按 UTF-8 字节计分别最多 32 和 191 字节，不能留空、带首尾空白或控制字符。gateway 的凭据上限为 128 字节，接入其他凭据格式时一起核对。注册准入回调在 DB 任务中运行，可能并发调用，只做检查。它不适合发奖、请求外部服务或再排一个数据库任务。[认证装配](../servers/account/server.go) · [账号查询与创建](../servers/account/internal/account/repository.go) · [网关凭据设置](../servers/gateway/authentication.go)

## 列区、选角和创建

| 客户端操作 | 谁处理 | 结果 |
| --- | --- | --- |
| `Account.ListZones` | account | 返回配置里的逻辑区编号、名称、开放状态。增加进程不改变此列表。 |
| `Roles.List` | account | 从数据库读取该账号、该区的角色摘要，包含角色状态。刷新列表不建角色。 |
| `Roles.Create` | account 校验区服，game 执行创建 | 创建档案和初始组件记录，不分配在线玩家归属。调用超时后刷新角色列表确认结果。 |
| `Player.Enter` | account 做准入检查，gateway 关联 game，game 选择或创建角色 | 首次进入成功返回完整状态。已有在线角色时须先离开才能更换角色或逻辑区。 |

客户端传逻辑区 ID，例如 `default`。关闭的逻辑区可以查看，不能创建或进入。配置见 [account.json](../configs/account.json)。指定 `role_id` 时，角色缺失、归属不符或角色被禁用均向客户端返回 `GAME-ROLE-FORBIDDEN`。传 0 时，没有角色就创建首个，恰有一个可用角色就进入，存在多个角色则返回 `GAME-CHOOSE-ROLE` 要求选择；仅有的角色被禁用也会拒绝进入。[目录与准入](../servers/account/internal/handler/directory.go) · [角色选择](../servers/game/internal/role/repository.go)

创建事务先锁账号行、确认账号正常，再按区检查数量。每区默认一个角色，`roles_per_zone` 可取 1–100，已禁用角色也计数。显示名允许空，非空最多 64 字节。不同创建请求没有业务去重编号；开放多角色时，每个成功的显式创建请求都可能产生新档案。

新角色的资料、三次挑战机会、五个初始经验道具在同一事务中创建，其中任何一项失败都会回滚。增加新手初始数据时，在初始化回调中使用传入的事务。不在加载已有角色时发放新手奖励。[角色初始化](../servers/game/internal/player/storage/initialize.go)
