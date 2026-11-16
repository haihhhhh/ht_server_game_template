package account

import (
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/account"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/directory"
	"github.com/haihhhhh/ht_server_game_template/servers/account/internal/handler"
	"github.com/topfreegames/pitaya/v2/component"
)

func registerRoutes(
	accountApp *app.App,
	accountManager *account.AccountManager,
	accountService *account.Service,
	directoryService *directory.Service,
	accountOwnerRegistry *placement.Registry,
) error {
	if err := accountApp.RegisterRemote(
		&handler.CredentialRPC{
			App:            accountApp,
			AccountService: accountService,
		},
		component.WithName("Credentials"),
	); err != nil {
		return err
	}
	return accountApp.RegisterRemote(
		&handler.AccountSessionRPC{
			App:              accountApp,
			AccountManager:   accountManager,
			AccountRegistry:  accountOwnerRegistry,
			DirectoryService: directoryService,
		},
		component.WithName("Accounts"),
	)
}
