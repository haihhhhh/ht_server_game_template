// 【通常无需修改｜启动】统一处理日志、启动和关闭；每个服务通过自己的 server.go 明确装配资源。增加玩法不改公共启动流程。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
// runtime 只管理各服务共用的进程资源，不导入任何服务私有实现。
package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"time"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/host"
	"github.com/haihhhhh/ht_server_frame/observability"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/config"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/topfreegames/pitaya/v2/tracing/jaeger"
)

func Run(serviceType string, baseConfig config.Base, build func(*Resources) (*app.App, error)) (runErr error) {
	if err := baseConfig.Validate(); err != nil {
		return err
	}
	if baseConfig.Instance == "" {
		baseConfig.Instance = serviceType + "-" + rand.Text()
	}
	logger, logOutput, err := logging.Open(baseConfig.Log, serviceType, baseConfig.Instance)
	if err != nil {
		return err
	}
	previousLogger := slog.Default()
	defer func() {
		slog.SetDefault(previousLogger)
		if closeErr := logOutput.Close(); closeErr != nil {
			slog.Error("日志文件关闭失败，请检查磁盘和文件系统状态。", "component", "logging")
		}
	}()
	slog.SetDefault(logger)
	if baseConfig.Trace {
		traceCloser, traceErr := jaeger.Configure(jaeger.Options{
			ServiceName: "ht-" + serviceType,
			Probability: .01,
		})
		if traceErr != nil {
			return traceErr
		}
		defer traceCloser.Close()
	}
	resources := &Resources{
		Logger:   logger,
		instance: serviceType + "/" + baseConfig.Instance,
	}
	started, stopped := false, false
	defer func() {
		if started && !stopped {
			// 收尾未确认时不拆除后台任务依赖，保留节点占用；最终由进程退出回收。
			return
		}
		if resources.Clock != nil {
			resources.Clock.Close()
		}
		if !started {
			resources.closeUnstarted()
		}
		if resources.idGenerator != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if closeErr := resources.idGenerator.Close(ctx); closeErr != nil {
				runErr = errors.Join(runErr, closeErr)
			}
			cancel()
		}
		resources.closeDiscovery()
	}()
	if err = resources.openClock(baseConfig); err != nil {
		return err
	}
	application, err := build(resources)
	if err != nil {
		return err
	}
	catalog, err := pb.NewProtocolCatalog()
	if err != nil {
		return err
	}
	if err = application.ConfigureProtocol(catalog, map[string]string{

		"Account": "gateway",

		"Roles": "gateway",

		"Player": "gateway",
	}); err != nil {
		return err
	}
	// Start 后若收尾未确认，由进程退出回收资源，不能提前关闭仍在使用的数据库。
	started = true
	if err = application.Start(context.Background()); err != nil {
		return err
	}
	logger.Info("服务已启动", "component", "runtime", "server_id", application.ServerID(), "address", application.Address())
	err = console(application, resources)
	stopped = err == nil
	return err
}
func NewApp(serviceType, address string, baseConfig config.Base, resources *Resources, entries ...host.Entry) (*app.App, error) {
	if resources.DB != nil {
		entries = append(entries, host.Entry{
			Name:   "database",
			Module: resources.DB,
		})
	}
	if resources.Data != nil {
		entries = append(entries, host.Entry{
			Name:   "tables",
			Module: resources.Data,
		})
	}
	if baseConfig.DiagnosticsAddress != "" {
		var collectors []prometheus.Collector
		var dependencies []string
		if resources.DB != nil {
			dependencies = []string{"database"}
			collectors = []prometheus.Collector{

				prometheus.NewGaugeFunc(
					prometheus.GaugeOpts{
						Name: "ht_save_pending",
						Help: "待完成数据库任务数",
					},
					func() float64 {
						return float64(resources.DB.Stats().Pending)
					},
				),

				prometheus.NewGaugeFunc(
					prometheus.GaugeOpts{
						Name: "ht_save_pending_bytes",
						Help: "待完成数据估计字节数",
					},
					func() float64 {
						return float64(resources.DB.Stats().Bytes)
					},
				),
			}
		}
		diagnosticsServer, err := observability.NewDiagnosticsServer(
			baseConfig.DiagnosticsAddress,
			baseConfig.DiagnosticsToken,
			baseConfig.Pprof,
			collectors...,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, host.Entry{
			Name:      "diagnostics",
			Module:    diagnosticsServer,
			DependsOn: dependencies,
		})
	}
	appConfig := app.DefaultConfig()
	appConfig.Address = address
	appConfig.ServerType = serviceType
	natsConnection, err := baseConfig.NATSConnection()
	if err != nil {
		return nil, err
	}
	clusterConfig := app.LocalClusterConfig{
		Frontend: serviceType == "gateway",
		NATS:     natsConnection,
		Etcd:     baseConfig.Etcd,
		Prefix:   baseConfig.Prefix,
	}
	if baseConfig.PrivateNetwork {
		return app.NewPrivateCluster(appConfig, clusterConfig, entries...)
	}
	return app.NewLocalCluster(appConfig, clusterConfig, entries...)
}
