package world

import (
	"time"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/host"
	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/runtime"
	"github.com/haihhhhh/ht_server_game_template/servers/world/internal/boss"
	"github.com/haihhhhh/ht_server_game_template/servers/world/internal/config"
	"github.com/topfreegames/pitaya/v2/component"
)

func Run() error {
	worldConfig, err := config.Read(base.Path())
	if err != nil {
		return err
	}
	return runtime.Run("world", worldConfig.Base, func(resources *runtime.Resources) (*app.App, error) {
		if openErr := resources.OpenDatabase(worldConfig.Base); openErr != nil {
			return nil, openErr
		}
		worldOwnerRegistry, buildErr := resources.Registry(worldConfig.Base, "bosses")
		if buildErr != nil {
			return nil, buildErr
		}

		// Boss 服务托管公共状态；网络入口在服务装配完成后登记。
		bossService := &boss.Service{
			DB:            resources.DB,
			WorldRegistry: worldOwnerRegistry,
			MaxScopes:     worldConfig.MaxScopes,
			QueueCapacity: worldConfig.QueueCapacity,
			SaveInterval:  time.Duration(worldConfig.SaveSeconds) * time.Second,
		}
		worldApp, buildErr := runtime.NewApp(
			"world",
			"",
			worldConfig.Base,
			resources,
			host.Entry{
				Name:      "world",
				Module:    bossService,
				DependsOn: []string{"database"},
			},
		)
		if buildErr != nil {
			return nil, buildErr
		}
		bossService.App = worldApp
		// TODO(业务扩展)：增加公共业务时，参照 BossRPC 注册内部入口，由对应服务管理自己的状态。
		buildErr = worldApp.RegisterRemote(
			&boss.BossRPC{
				App:         worldApp,
				BossService: bossService,
			},
			component.WithName("Boss"),
		)
		return worldApp, buildErr
	})
}
