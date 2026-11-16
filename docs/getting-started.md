# 从源码启动

## 准备四个平级目录

```text
工作目录/
  ht_server_frame/
  ht_server_game_template/
  ht_server_game_protocol/
  ht_server_game_tables/
```

模板通过 `go.mod` 的 `replace` 读取框架。另两个仓库分别提供配套的 `game.proto`（账号、角色 ID 为 `int64`）、Luban 配置和源表。从模板根目录运行以下命令。

## 生成和构建

构建机准备 Go 1.27.2、本平台的 protoc 25.1、Luban 5.1.0 和 .NET 8。下面的 Windows CMD 示例假设 Go 和 .NET 在 PATH 中，`tools` 是你自行准备的平级工具目录；根据实际位置修改路径。

```bat
set "LUBAN_DLL=%CD%\..\tools\luban-5.1.0\Luban\Luban.dll"
scripts\Install-ProtocolTools.bat "..\tools\protoc\bin\protoc.exe"
scripts\Generate-Protocol.bat
scripts\Generate-Tables.bat
scripts\Build.bat
```

Linux 运行相同名称的脚本，使用 Linux 版 protoc：

```sh
export LUBAN_DLL="$PWD/../tools/luban-5.1.0/Luban/Luban.dll"
sh scripts/Install-ProtocolTools.sh ../tools/protoc/bin/protoc
sh scripts/Generate-Protocol.sh
sh scripts/Generate-Tables.sh
sh scripts/Build.sh
```

每步成功后再继续。`GO_EXE`、`DOTNET_EXE` 可以指定工具位置。安装脚本把 protoc 复制到模板的 `.tools/protobuf/bin` 并准备固定版本的 protoc-gen-go v1.31.0。`Build` 生成玩家模块导入文件，编译 gateway、account、game、world、client 并执行 `go vet ./...`。如使用 IDE 构建，先生成协议和表格，再运行 `go run ./tools/moduleimports`。

## 准备基础服务

- 启动 NATS 和 etcd。模板样例使用 `nats://127.0.0.1:14222` 和 `http://127.0.0.1:12379`。Windows 开发环境可先按框架指南生成配置并手工启动 etcd，再用模板配置启动有四个服务用户的 NATS。[本机 etcd 步骤](https://github.com/haihhhhh/ht_server_frame/blob/master/docs/implementation/local-services.md) · [模板 NATS 配置](nats.md)
- 在 MySQL 创建开发库，例如 `CREATE DATABASE ht_game_dev CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`。`account`、`game` 和 `world` 使用同一库，启动时按模型迁移表，数据库用户需要相应的 DDL 和读写权限。为它们设置 `HT_MYSQL_DSN`，格式示意为 `用户:密码@tcp(地址:端口)/ht_game_dev?parseTime=true&charset=utf8mb4`。
- 各服务须使用同一环境的 NATS、etcd 和 `prefix`。每个服务进程单独设置 `HT_NATS_USER` 为服务名、`HT_NATS_PASSWORD` 为其在 NATS 上的密码。`account` 和示例客户端还需设置相同的 `HT_DEV_TICKET`（16–128 字节）。此票据只对应一个固定开发账号，正式项目在 account 入口替换验证器。
- 确认 `account` 和 `game` 的 `id_node` 不重复。样例分别使用 1 和 2，其他发号进程也要分配独立编号。见[配置说明](service-config.md)。

## 运行

从模板根目录分别在三个终端执行，每个终端使用对应服务的环境变量：

```bat
scripts\Start.bat account configs\account.json
scripts\Start.bat game configs\game.json
scripts\Start.bat gateway configs\gateway.json
```

在客户端终端执行 `bin\client.exe -zone default -use-exp-item 1`。需要挑战 Boss 时，在另一个终端运行 `scripts\Start.bat world configs\world.json` 并为客户端添加 `-boss`。Linux 分别使用 `sh scripts/Start.sh <服务> configs/<服务>.json` 和 `./bin/client`。更多操作见[示例客户端](sample-client.md)。

在服务终端输入 `status` 查看运行状态，输入 `stop` 正常停服。运行机需要服务程序、配置、game 的完整表包及基础服务连接，部署步骤见[运行手册](operations.md)。
