package account

import (
	"time"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/host"
	base "github.com/haihhhhh/ht_server_game_template/infrastructure/config"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/runtime"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/account"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/config"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/directory"
)

func Run() error {
	accountConfig, err := config.Read(base.Path())
	if err != nil {
		return err
	}
	// TODO(业务扩展)：接入正式账号渠道时，在这里提供凭据验证函数，参照 DevelopmentVerifier 的签名。
	verifyCredential, err := account.DevelopmentVerifier(accountConfig.DevTicket)
	if err != nil {
		return err
	}
	return runtime.Run("account", accountConfig.Base, func(resources *runtime.Resources) (*app.App, error) {
		idGenerator, buildErr := resources.OpenIDGenerator(accountConfig.Base)
		if buildErr != nil {
			return nil, buildErr
		}
		if openErr := resources.OpenDatabase(accountConfig.Base); openErr != nil {
			return nil, openErr
		}
		accountOwnerRegistry, buildErr := resources.Registry(accountConfig.Base, "account-owners")
		if buildErr != nil {
			return nil, buildErr
		}
		// AccountManager 托管在线账号；验证凭据和查询目录使用同一个数据库服务。
		accountManager := &account.AccountManager{
			DB:              resources.DB,
			AccountRegistry: accountOwnerRegistry,
			MaxAccounts:     accountConfig.MaxAccounts,
			QueueCapacity:   accountConfig.QueueCapacity,
			SaveInterval:    time.Duration(accountConfig.SaveSeconds) * time.Second,
		}
		accountApp, buildErr := runtime.NewApp(
			"account",
			"",
			accountConfig.Base,
			resources,
			host.Entry{
				Name:      "accounts",
				Module:    accountManager,
				DependsOn: []string{"database"},
			},
		)
		if buildErr != nil {
			return nil, buildErr
		}
		accountService := &account.Service{
			IDGenerator: idGenerator,
			DB:          resources.DB,
			Verify:      verifyCredential,
		}
		directoryService := &directory.Service{
			DB:    resources.DB,
			Zones: accountConfig.Zones,
		}
		return accountApp, registerRoutes(accountApp, accountManager, accountService, directoryService, accountOwnerRegistry)
	})
}
