// 【通常无需修改｜资源装配】提供数据库、表格、归属与发号器的创建方法；是否需要这些资源由各服务入口决定。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package runtime

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/haihhhhh/ht_server_frame/gameclock"
	"github.com/haihhhhh/ht_server_frame/idgen"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_frame/tables"
	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/config"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Resources 只在装配阶段使用；服务明确调用所需创建方法，不按服务名推测资源。
type Resources struct {
	Clock       *gameclock.Clock
	instance    string
	idGenerator *idgen.Generator
	DB          *database.Service
	Data        *tables.Module[gamedata.Data]
	Logger      *slog.Logger
	discovery   *clientv3.Client
}

func (resources *Resources) openClock(baseConfig config.Base) error {
	client, err := resources.discoveryClient(baseConfig)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resources.Clock, err = gameclock.Open(ctx, client, "/ht-game/"+baseConfig.Prefix+"/clock/offset")
	return err
}

func (resources *Resources) OpenDatabase(baseConfig config.Base) error {
	if resources.DB != nil {
		return nil
	}
	if err := baseConfig.ValidateDatabase(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	resources.DB, err = database.Open(ctx, baseConfig.DSN, baseConfig.DBQueues, resources.Clock.Now)
	return err
}

func (resources *Resources) OpenTables(baseConfig config.Base) error {
	if resources.Data != nil {
		return nil
	}
	var err error
	resources.Data, err = gamedata.New(baseConfig.Tables)
	return err
}

func (resources *Resources) discoveryClient(baseConfig config.Base) (*clientv3.Client, error) {
	if resources.discovery == nil {
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{baseConfig.Etcd},
			DialTimeout: 3 * time.Second,
		})
		if err != nil {
			return nil, err
		}
		resources.discovery = client
	}
	return resources.discovery, nil
}

func (resources *Resources) OpenIDGenerator(baseConfig config.Base) (*idgen.Generator, error) {
	if resources.idGenerator != nil {
		return nil, errors.New("发号器已装配")
	}
	client, err := resources.discoveryClient(baseConfig)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	generator, err := idgen.Open(ctx, client, baseConfig.IDNode, resources.instance)
	if err != nil {
		return nil, err
	}
	resources.idGenerator = generator
	return generator, nil
}

func (resources *Resources) Registry(baseConfig config.Base, name string) (*placement.Registry, error) {
	client, err := resources.discoveryClient(baseConfig)
	if err != nil {
		return nil, err
	}
	return placement.New(client, "/ht-game/"+baseConfig.Prefix+"/"+name, 3*time.Second)
}

// 装配失败时尚未交给 host；启动后由 host 关闭 DB，etcd 在业务停止后关闭。
func (resources *Resources) closeUnstarted() {
	if resources.DB != nil {
		if err := resources.DB.Close(); err != nil {
			resources.Logger.Error("数据库资源关闭失败", "component", "runtime")
		}
	}
}

func (resources *Resources) closeDiscovery() {
	if resources.discovery != nil {
		if err := resources.discovery.Close(); err != nil {
			resources.Logger.Error("归属客户端关闭失败", "component", "runtime")
		}
	}
}
