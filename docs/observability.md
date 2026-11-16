# 日志与诊断

四个服务在读取配置前把 JSON 日志写到标准错误，实例字段为 `starting`。读取配置后切换为包含 `service` 和 `instance` 的业务日志；启动成功时还能看到本次 Pitaya 的 `server_id`。默认格式 JSON、等级 Info、控制台开启，文件输出关闭。

## 配置业务日志

将下列 `log` 字段加入服务的 JSON。给每个写日志的进程分配独立文件，例中的路径相对于进程工作目录。

```json
{
  "log": {
    "level": "info",
    "format": "json",
    "console": true,
    "source": false,
    "file": "logs/game-1.log",
    "max_mb": 20,
    "backups": 3
  }
}
```

`format` 支持 `json`、`text`；`HT_LOG_LEVEL` 可覆盖业务等级。文件达到轮转条件时，保留当前文件和至多 `backups` 个备份。`max_mb` 范围 1–1024 MiB，`backups` 为 1–20。文件打不开会导致启动失败；运行中写入失败时尝试向标准错误输出，输出故障提示每分钟至多一次。日志为同步写入，慢磁盘会延长调用时间。

请求处理中用 `observability.Logger(ctx)` 保留 `request_id`，按 `component` 和 `stage` 查找同一请求或一次保存的执行过程。周期保存有自己的批次标识。入队日志说明队列已接收任务；要判断数据库写入，请看执行结果。Pitaya 使用独立的 logrus 日志器，默认 Info、写标准错误，业务日志等级和文件轮转配置只作用于 slog。

## 开启监控和采样

1. 给要监控的进程设置 `HT_DIAGNOSTICS_ADDR=127.0.0.1:9091` 和 32–512 字节的 `HT_DIAGNOSTICS_TOKEN`，重启进程。每个实例使用独立的回环端口。未设置地址时没有诊断监听。
2. 向 `/metrics` 发 GET 请求，带 `Authorization: Bearer <令牌>`。包括 Go、进程和请求指标；打开 DB 的服务还提供 `ht_save_pending` 和 `ht_save_pending_bytes`。这两个值统计待完成的数据库任务及估算字节，含查询和写入。
3. 用[Prometheus 样例](../ops/monitoring/prometheus.yml)时，将同一令牌保存在配置旁的 `diagnostics-token.local`，调整 `targets` 为实际可达的回环端口。样例每 15 秒抓取一次；[规则](../ops/monitoring/alerts.yml)中 `GameMetricsUnavailable` 在抓取失败条件持续 1 分钟后触发，抓取恢复后解除。已有 Grafana 可导入[样例看板](../ops/monitoring/dashboard.json)。
4. 需要诊断性能时，在同一进程设置 `HT_PPROF=1` 启用 `/debug/pprof/`。接口同样需要凭据，诊断服务最多并行处理两个已认证请求。CPU profile 与 trace 默认采样 5 秒，可指定 `seconds=1` 至 `5`。采样文件可能包含业务内存，应限制访问。

## 链路追踪

设置 `HT_TRACE=1` 后，服务调用 Pitaya 的 Jaeger/OpenTracing 配置，服务名为 `ht-gateway`、`ht-account`、`ht-game` 或 `ht-world`，采样概率 1%。部署环境需另行提供兼容的 Jaeger 接收端。排查一条具体失败请求时，先按日志中的 `request_id` 关联各服务，再查看有采样记录的链路。
