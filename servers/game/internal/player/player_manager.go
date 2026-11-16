// 【通常无需修改｜玩家循环、S01】管理在线玩家、进入退出和周期调度；新玩法放到 modules，组件变化交给统一保存。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package player

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/execution"
	"github.com/haihhhhh/ht_server_frame/gameclock"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_frame/tables"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/database"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/role"
)

var ErrBusy = errcode.ErrPlayerBusy

// loading 和 retainUntilRelease 由 PlayerManager.mu 保护；closed 关闭后才能读取 saved。
type onlinePlayerEntry struct {
	state              *Player
	loopKey            string // 执行器按字符串区分业务循环，创建时转换一次。
	loop               *execution.Executor[*Player]
	loading            bool
	leaving            atomic.Bool // 退出流程在管理器锁内置位，玩家 Tick 可独立读取。
	retainUntilRelease bool
	closed             chan struct{}
	saved              bool
}

// PlayerManager 按账号管理当前连接绑定，以及该账号进入的角色及其业务循环。
// 账号可以先绑定连接再选角，因此有绑定时不一定已经有在线 Player。
type PlayerManager struct {
	ConfigTables           *tables.Module[gamedata.Data]
	Clock                  *gameclock.Clock
	App                    *app.App
	WorldRegistry          *placement.Registry
	MaxPlayers             int
	QueueCapacity          int
	TickInterval           time.Duration
	SaveInterval           time.Duration
	mu                     sync.RWMutex
	DB                     *database.Service
	RoleService            *role.Service
	onlinePlayersByAccount map[int64]*onlinePlayerEntry
	bindingsByAccount      map[int64]playerBinding
	closing                bool
}

func (playerManager *PlayerManager) Init(context.Context) error {
	if playerManager.Clock == nil || playerManager.MaxPlayers < 1 || playerManager.QueueCapacity < 1 ||
		playerManager.TickInterval <= 0 || playerManager.SaveInterval <= 0 {
		return errors.New("Player: invalid loop limits")
	}
	playerManager.onlinePlayersByAccount = make(map[int64]*onlinePlayerEntry)
	playerManager.bindingsByAccount = make(map[int64]playerBinding)
	return nil
}

// 执行时再检查一次，等待中的请求可能遇到换连接或退出。Enter 自身可以在加载中执行。
func (playerManager *PlayerManager) checkQueued(account int64, binding *playerBinding, playerEntry *onlinePlayerEntry) error {
	playerManager.mu.RLock()
	defer playerManager.mu.RUnlock()
	if err := playerManager.checkBindingLocked(account, binding); err != nil {
		return err
	}
	if playerManager.onlinePlayersByAccount[account] != playerEntry || playerEntry.leaving.Load() || (binding == nil && playerEntry.loading) {
		return ErrBusy
	}
	return nil
}

func (playerManager *PlayerManager) newOnlinePlayerEntry(account int64) (*onlinePlayerEntry, error) {
	playerEntry := &onlinePlayerEntry{
		state:   &Player{},
		loopKey: clusterlink.AccountUID(account),
		loading: true,
		closed:  make(chan struct{}),
	}
	loop, err := execution.New(execution.Options[*Player]{
		Name:          "Player",
		MaxDomains:    1,
		QueueCapacity: playerManager.QueueCapacity,
		MaxQueueBytes: 1 << 20,
		InitTimeout:   3 * time.Second,
		Init: func(context.Context, string) (*Player, error) {
			return playerEntry.state, nil
		},
		UpdateInterval: min(playerManager.TickInterval, playerManager.SaveInterval),
		Update:         playerEntry.update,
	})
	playerEntry.loop = loop
	return playerEntry, err
}

func (playerManager *PlayerManager) Enter(ctx context.Context, account int64, req *pb.EnterReq) (*pb.PlayerState, error) {
	binding, err := bindingFromContext(ctx)
	if err != nil {
		return nil, err
	}
	zone, roleID := req.Zone, req.RoleId
	playerManager.mu.Lock()
	if err = playerManager.checkBindingLocked(account, &binding); err != nil {
		playerManager.mu.Unlock()
		return nil, err
	}
	playerEntry := playerManager.onlinePlayersByAccount[account]
	fresh := playerEntry == nil
	if fresh {
		if len(playerManager.onlinePlayersByAccount) >= playerManager.MaxPlayers {
			playerManager.mu.Unlock()
			return nil, ErrBusy
		}
		playerEntry, err = playerManager.newOnlinePlayerEntry(account)
		if err != nil {
			playerManager.mu.Unlock()
			return nil, err
		}
		playerManager.onlinePlayersByAccount[account] = playerEntry
	} else if playerEntry.loading || playerEntry.leaving.Load() {
		playerManager.mu.Unlock()
		return nil, ErrBusy
	}
	playerManager.mu.Unlock()
	out, err := playerEntry.loop.Call(ctx, playerEntry.loopKey, int64(len(zone))+256, func(task context.Context, onlinePlayer *Player) (any, error) {
		if checkErr := playerManager.checkQueued(account, &binding, playerEntry); checkErr != nil {
			return nil, checkErr
		}
		if !onlinePlayer.ready {
			if loadErr := playerManager.load(task, account, zone, roleID, onlinePlayer); loadErr != nil {
				return nil, loadErr
			}
			if checkErr := playerManager.checkQueued(account, &binding, playerEntry); checkErr != nil {
				return nil, checkErr
			}
		} else if refreshErr := onlinePlayer.components.RefreshConfig(); refreshErr != nil {
			return nil, refreshErr
		}
		if zone != onlinePlayer.profile.Role().Zone || (roleID != 0 && roleID != onlinePlayer.profile.Role().ID) {
			return nil, errors.New("Player: leave current role first")
		}
		return onlinePlayer.components.Current(), nil
	})
	if err != nil {
		// 首次进入失败时保持加载中，直到退出流程关闭准入。
		if fresh || errors.Is(err, execution.ErrFaulted) {
			playerManager.release(account, playerEntry)
		}
		return nil, err
	}
	if fresh {
		playerManager.mu.Lock()
		playerEntry.loading = false
		playerManager.mu.Unlock()
	}
	return out.(*pb.PlayerState), nil
}

// Leave 由已核对绑定的会话退出任务调用，也供正常停服复用；不依赖 RPC 身份的有效期。
func (playerManager *PlayerManager) Leave(ctx context.Context, account int64, binding clusterlink.Binding) (bool, error) {
	playerManager.mu.Lock()
	current, bound := playerManager.bindingsByAccount[account]
	if !bound || current.connection != binding {
		playerManager.mu.Unlock()
		return false, access.ErrUnauthenticated
	}
	playerEntry := playerManager.onlinePlayersByAccount[account]
	if playerEntry != nil {
		playerEntry.retainUntilRelease = true
	}
	playerManager.mu.Unlock()
	if playerEntry == nil {
		return true, nil
	}
	playerManager.release(account, playerEntry)
	select {
	case <-playerEntry.closed:
		return playerEntry.saved, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// release 先关闭准入并等循环结束，最后保存不受网络请求取消影响。
// 不在循环内调用等待自身退出；每个有容量限制的玩家最多启动一个退出等待者。
func (playerManager *PlayerManager) release(account int64, playerEntry *onlinePlayerEntry) {
	playerManager.mu.Lock()
	if playerEntry.leaving.Load() {
		playerManager.mu.Unlock()
		return
	}
	playerEntry.leaving.Store(true)
	playerManager.mu.Unlock()
	playerEntry.loop.BeginStop()
	go func() {
		defer func() {
			if recover() != nil {
				logging.Panic("player_release")
			}
		}()
		// 必须等实际执行结束才能释放，超时不能创建同一玩家的第二个循环。
		_ = playerEntry.loop.Stop(context.Background())
		// 循环已停止，始终唤醒等待者；正常下线条目保留到会话层确认归属释放。
		defer func() {
			playerManager.mu.Lock()
			if !playerEntry.retainUntilRelease && playerManager.onlinePlayersByAccount[account] == playerEntry {
				delete(playerManager.onlinePlayersByAccount, account)
			}
			playerManager.mu.Unlock()
			close(playerEntry.closed)
		}()
		var err error
		if playerEntry.loop.Stats().Panics > 0 {
			err = errors.New("Player: panic; discard possibly partial state")
		} else if playerEntry.state.ready {
			err = playerManager.finish(playerEntry.state)
		}
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = database.WaitForSave(waitCtx, playerEntry.state.saver.LastSubmitted, err)
		if err != nil {
			logger := slog.With("component", "player", "stage", "leave", "account_id", account)
			if playerEntry.state.ready && playerEntry.state.profile != nil {
				logger = logger.With("role_id", playerEntry.state.profile.Role().ID)
			}
			logger.Error("退出保存未确认，按策略释放", "kind", "save_unconfirmed")
		}
		playerEntry.saved = err == nil
	}()
}

func (playerManager *PlayerManager) Close() error {
	playerManager.mu.Lock()
	playerManager.closing = true
	list := make(map[int64]*onlinePlayerEntry, len(playerManager.onlinePlayersByAccount))
	for key, playerEntry := range playerManager.onlinePlayersByAccount {
		list[key] = playerEntry
	}
	playerManager.mu.Unlock()
	for key, playerEntry := range list {
		playerManager.release(key, playerEntry)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for _, playerEntry := range list {
		select {
		case <-playerEntry.closed:
		case <-timer.C:
			return errors.New("Player: close timeout")
		}
	}
	return nil
}

// BeginStop 在会话开始收尾时停止玩家准入，已接受的退出任务仍可完成。
func (playerManager *PlayerManager) BeginStop() {
	playerManager.mu.Lock()
	defer playerManager.mu.Unlock()
	playerManager.closing = true
}

// finish 在循环退出后执行；模块钩子异常不能阻止玩家索引释放。
func (playerManager *PlayerManager) finish(onlinePlayer *Player) (err error) {
	defer func() {
		if recover() != nil {
			logging.Panic("player_leave")
			err = errors.New("Player: leave hook panic")
		}
	}()
	onlinePlayer.components.Leave(playerManager.Clock.Now())
	return playerManager.save(onlinePlayer, true)
}
