package account

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/execution"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"

	"gorm.io/gorm"
)

var ErrBusy = errcode.ErrAccountBusy

type accountSlot struct {
	op               sync.Mutex
	state            *Account
	loop             *execution.Executor[*Account]
	assignment       placement.Assignment
	releaseOnExit    bool // 由 op 保护，只在本次归属已确认或登记结果未知时为真。
	loading, leaving bool // 由 AccountManager.mu 保护。
	closed           chan struct{}
}

// AccountManager 管理本节点在线账号；DB 等待不持有索引锁，也不占用账号循环。
type AccountManager struct {
	DB                         *database.Service
	AccountRegistry            *placement.Registry
	MaxAccounts, QueueCapacity int
	SaveInterval               time.Duration
	mu                         sync.Mutex
	users                      map[int64]*accountSlot
	closing                    bool
}

func (accountManager *AccountManager) Init(context.Context) error {
	if accountManager.DB == nil || accountManager.AccountRegistry == nil ||
		accountManager.MaxAccounts < 1 || accountManager.QueueCapacity < 1 ||
		accountManager.SaveInterval <= 0 {
		return errors.New("account: invalid manager configuration")
	}
	accountManager.users = make(map[int64]*accountSlot)
	return nil
}

// slot 连同账号操作锁一起交给 Attach；断线能等到首次归属申请结束再核对绑定。
func (accountManager *AccountManager) slot(accountID int64, assignment placement.Assignment) (*accountSlot, bool, error) {
	accountManager.mu.Lock()
	defer accountManager.mu.Unlock()
	if accountManager.closing {
		return nil, false, ErrBusy
	}
	if accountEntry := accountManager.users[accountID]; accountEntry != nil {
		if accountEntry.loading || accountEntry.leaving {
			return nil, false, ErrBusy
		}
		if accountEntry.assignment.AllocationID != assignment.AllocationID {
			return nil, false, access.ErrForbidden
		}
		if !accountEntry.op.TryLock() {
			return nil, false, ErrBusy
		}
		return accountEntry, false, nil
	}
	if len(accountManager.users) >= accountManager.MaxAccounts {
		return nil, false, ErrBusy
	}
	accountEntry := &accountSlot{
		state:      &Account{},
		assignment: assignment,
		loading:    true,
		closed:     make(chan struct{}),
	}
	loop, err := execution.New(execution.Options[*Account]{
		Name:          "Account",
		MaxDomains:    1,
		QueueCapacity: accountManager.QueueCapacity,
		MaxQueueBytes: 1 << 20,
		InitTimeout:   3 * time.Second,
		Init: func(context.Context, string) (*Account, error) {
			return accountEntry.state, nil
		},
		UpdateInterval: accountManager.SaveInterval,
		Update: func(_ context.Context, onlineAccount *Account) error {
			return accountManager.save(onlineAccount)
		},
	})
	if err != nil {
		return nil, false, err
	}
	accountEntry.loop = loop
	accountEntry.op.Lock()
	accountManager.users[accountID] = accountEntry
	return accountEntry, true, nil
}

func (accountManager *AccountManager) current(id int64) (*accountSlot, error) {
	accountManager.mu.Lock()
	defer accountManager.mu.Unlock()
	accountEntry := accountManager.users[id]
	if accountManager.closing || accountEntry == nil || accountEntry.loading || accountEntry.leaving {
		return nil, ErrBusy
	}
	return accountEntry, nil
}

// Attach 首次在循环外加载，结果回原对象；替换只改连接，不重新读库覆盖在线状态。
func (accountManager *AccountManager) Attach(ctx context.Context, assignment placement.Assignment, binding clusterlink.Binding, claim bool) (clusterlink.Reply, error) {
	if assignment.Scope != binding.Identity.UserID {
		return clusterlink.Reply{}, access.ErrForbidden
	}
	accountID, err := clusterlink.ParseAccountID(binding.Identity.UserID)
	if err != nil {
		return clusterlink.Reply{}, err
	}
	accountEntry, fresh, err := accountManager.slot(accountID, assignment)
	if err != nil {
		if claim && errors.Is(err, access.ErrForbidden) {
			// 两个网关可能选中同一节点。只返回仍在 etcd 上的实际归属。
			owner, found, lookupErr := accountManager.AccountRegistry.Lookup(ctx, assignment.Scope)
			if lookupErr != nil {
				return clusterlink.Reply{}, lookupErr
			}
			if found && owner.AllocationID != assignment.AllocationID {
				return clusterlink.Reply{Redirect: &owner}, nil
			}
		}
		return clusterlink.Reply{}, err
	}
	defer accountEntry.op.Unlock()

	var owner placement.Assignment
	var found bool
	if fresh && claim {
		// 本机已给这个账号留出名额；只有赢得归属后才读取账号数据。
		accountEntry.releaseOnExit = true
		owner, err = accountManager.AccountRegistry.Claim(ctx, assignment)
		found = err == nil
	} else {
		owner, found, err = accountManager.AccountRegistry.Lookup(ctx, assignment.Scope)
	}
	ownsAssignment := found && owner.ServerID == assignment.ServerID && owner.AllocationID == assignment.AllocationID
	if fresh && err == nil {
		accountEntry.releaseOnExit = ownsAssignment
	}
	if err != nil || !ownsAssignment {
		if fresh {
			accountManager.release(accountID, accountEntry)
		}
		if err != nil {
			return clusterlink.Reply{}, err
		}
		if claim && found && owner.AllocationID != assignment.AllocationID {
			return clusterlink.Reply{Redirect: &owner}, nil
		}
		return clusterlink.Reply{}, access.ErrForbidden
	}
	var loaded model.Account
	if fresh {
		loaded, err = database.Query(ctx, accountManager.DB, database.AccountKey(accountID), 256, func(db *gorm.DB) (model.Account, error) {
			var value model.Account
			err := db.First(&value, "id = ?", accountID).Error
			return value, err
		})
		if err != nil {
			accountManager.release(accountID, accountEntry)
			return clusterlink.Reply{}, err
		}
	}
	out, err := accountEntry.loop.Call(ctx, assignment.Scope, 512, func(_ context.Context, onlineAccount *Account) (any, error) {
		if fresh {
			onlineAccount.data = loaded
			onlineAccount.submittedStatus = loaded.Status
		}
		if onlineAccount.data.Status != model.Normal {
			return nil, ErrBanned
		}
		if onlineAccount.binding.Gateway == binding.Gateway && onlineAccount.binding.Identity.Generation > binding.Identity.Generation {
			return nil, access.ErrUnauthenticated
		}
		previous := onlineAccount.binding
		if previous.SameConnection(binding) {
			return clusterlink.Reply{Current: previous}, nil
		}
		if previous.Order.Sequence == ^uint64(0) {
			return nil, ErrBusy
		}
		binding.Order = clusterlink.LoginOrder{
			OwnerRevision: owner.Revision,
			Sequence:      previous.Order.Sequence + 1,
		}
		onlineAccount.binding = binding
		return clusterlink.Reply{
			Previous: previous,
			Current:  binding,
		}, nil
	})
	if fresh {
		if err != nil {
			accountManager.release(accountID, accountEntry)
		} else {
			accountManager.mu.Lock()
			accountEntry.loading = false
			accountManager.mu.Unlock()
		}
	}
	if err != nil {
		return clusterlink.Reply{}, err
	}
	reply := out.(clusterlink.Reply)
	reply.Assignment = &owner
	return reply, nil
}

// Call 不要求客户端 Session，管理入口可以在自己的鉴权完成后复用。
func (accountManager *AccountManager) Call(ctx context.Context, id int64, bytes int64, fn func(context.Context, *Account) (any, error)) (any, error) {
	if fn == nil {
		return nil, errors.New("account: operation required")
	}
	accountEntry, err := accountManager.current(id)
	if err != nil {
		return nil, err
	}
	return accountEntry.loop.Call(ctx, accountEntry.assignment.Scope, bytes, func(task context.Context, onlineAccount *Account) (any, error) {
		return fn(task, onlineAccount)
	})
}

func (accountManager *AccountManager) Disconnect(ctx context.Context, allocation string, binding clusterlink.Binding) error {
	id, err := clusterlink.ParseAccountID(binding.Identity.UserID)
	if err != nil {
		return err
	}
	accountManager.mu.Lock()
	accountEntry := accountManager.users[id]
	accountManager.mu.Unlock()
	if accountEntry == nil || accountEntry.assignment.AllocationID != allocation {
		// 容量拒绝或旧分配已经释放，这次通知不需要创建对象，也不碰其他分配。
		return nil
	}
	accountEntry.op.Lock()
	accountManager.mu.Lock()
	leaving := accountEntry.leaving
	accountManager.mu.Unlock()
	if !leaving {
		// 若通知恰好在首次登录中到达，仍要在绑定完成后处理，独立短期限只用于核对当前连接。
		check, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		out, err := accountEntry.loop.Call(check, accountEntry.assignment.Scope, 512, func(_ context.Context, onlineAccount *Account) (any, error) {
			return onlineAccount.binding.SameConnection(binding), nil
		})
		cancel()
		if err != nil || !out.(bool) {
			accountEntry.op.Unlock()
			return err
		}
		accountManager.release(id, accountEntry)
	}
	accountEntry.op.Unlock()
	select {
	case <-accountEntry.closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// save 只写账号负责的状态字段，不覆盖角色创建或其他服务的数据。
func (accountManager *AccountManager) save(onlineAccount *Account) error {
	if onlineAccount.data.ID <= 0 || onlineAccount.data.Status == onlineAccount.submittedStatus {
		return nil
	}
	id, status := onlineAccount.data.ID, onlineAccount.data.Status
	done, err := accountManager.DB.Submit(context.Background(), database.AccountKey(id), 128, func(db *gorm.DB) error {
		return db.Model(&model.Account{}).Where("id = ?", id).Update("status", status).Error
	})
	if err == nil {
		onlineAccount.submittedStatus = status
		onlineAccount.lastSave = done
	} else {
		slog.Error("账号保存未入队", "component", "account", "stage", "enqueue")
	}
	return err
}

func (accountManager *AccountManager) Close() error {
	accountManager.mu.Lock()
	accountManager.closing = true
	list := make(map[int64]*accountSlot, len(accountManager.users))
	for id, accountEntry := range accountManager.users {
		list[id] = accountEntry
	}
	accountManager.mu.Unlock()
	for id, accountEntry := range list {
		accountManager.release(id, accountEntry)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for _, accountEntry := range list {
		select {
		case <-accountEntry.closed:
		case <-timer.C:
			return errors.New("account: close timeout")
		}
	}
	return nil
}
