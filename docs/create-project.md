# 从模板创建项目

创建工具从当前工作区复制服务端、协议和表格三个源目录，为新项目建立三个独立 Git 仓库。框架仍使用同一份平级源码。准备好 Git、Go 及[本地构建工具](getting-started.md)，先在模板运行 `Install-ProtocolTools`，再从模板根目录执行：

```bat
scripts\Create-Project.bat git.example.com/game/mygame
```

```sh
sh scripts/Create-Project.sh git.example.com/game/mygame
```

替换示例中的 module 名称，省去 `https://` 前缀。命令将创建 `mygame`、`mygame_protocol` 和 `mygame_tables`；以 `/v2` 等主版本号结尾时，用前一段作为目录名。三个目标目录都须尚未存在。

工具复制 Git 已跟踪文件的当前内容及未被忽略的新文件，跳过已删除文件。下载 ZIP 时也按源目录的忽略规则取文件。它会替换文本中的 module 和共享仓库引用，复制已准备好的两个协议工具，重新生成协议和表格，检查旧 module 的残留，再用 `Build` 生成模块导入、编译和静态检查。新仓库没有自动提交和远端。

如果复制中途失败，检查已创建目录，确认其中的文件可删除后再重新创建。如果源码复制完整而生成或编译失败，可在**新服务端目录**修正工具配置，从报错步骤继续运行 `Generate-Protocol`、`Generate-Tables` 或 `Build`。模板后续修改不会自动同步到已创建的项目。
