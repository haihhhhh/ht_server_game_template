# 用示例客户端走一次登录到下线

按[从源码启动](getting-started.md)生成并启动服务，在客户端终端设置与 account 相同的 `HT_DEV_TICKET`。默认网关为 `ws://127.0.0.1:3250/`，逻辑区为 `default`。下面是 Windows 命令，Linux 将 `bin\client.exe` 换为 `./bin/client`。

| 使用场景 | 命令 | 实际操作 |
| --- | --- | --- |
| 只查看目录 | `bin\client.exe -directory-only` | 登录，列出逻辑区和所选区的角色，关闭连接。不会调用 Enter 或创建角色。 |
| 正常进入并离开 | `bin\client.exe` | 登录并查目录，进入角色，将偏好改为 `sample-setting`，发送 Leave 并打印保存确认。无角色时可在进入该区时自动建角。 |
| 使用经验道具 | `bin\client.exe -use-exp-item 1` | 在进入后使用一个初始经验道具，打印剩余数量、经验和等级。重复运行会继续消耗当前角色已有的道具。 |
| 挑战 Boss | `bin\client.exe -boss` | 需要 world 在线。进入后挑战一次，打印 Boss 剩余血量和玩家剩余次数。 |

使用 `-url` 选另一网关，`-zone` 指定区服。例如 `bin\client.exe -url ws://127.0.0.1:3251/ -zone default -directory-only`。同一开发票据仍对应同一账号。`-boss` 和 `-use-exp-item` 可以并用，此时先攻击，再用道具。

客户端按请求序号对应响应，遇到主动推送则显示消息类型并继续等响应。业务错误打印服务端返回的错误码，本次运行随即结束。Leave 返回选角后示例程序会关闭 WebSocket；这次断开才触发账号下线。源码见 [cmd/client/main.go](../cmd/client/main.go)。
