package connection

import (
	"context"
	"crypto/rand"
	"errors"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/host"
	"github.com/haihhhhh/ht_server_frame/observability"
	"github.com/haihhhhh/ht_server_frame/persistence"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
)

const (
	loginReserved uint32 = iota
	loginInProgress
	loginReady
)

// DisconnectQueueCount 同时决定按账号分配的队列数和网关为它们保留的 RPC 名额。
const DisconnectQueueCount = 4

type connection struct {
	slot    *accountEntry
	binding clusterlink.Binding
	login   atomic.Uint32
	// 只在当前连接收到成功的 Enter 回包后置位，由账号的 turn 保护。新连接从头确认。
	enteredRole bool
}

// 每个在线账号占一个下线名额。换连接只更新 current，两个后端分别确认后才交接。
type accountEntry struct {
	current *connection   // 由 Manager.mu 保护，预留和主动推送都不等后端 RPC。
	turn    chan struct{} // 同账号串行；等待中的请求可以按自己的期限退出。
	account backendLink
	game    backendLink
}

type Manager struct {
	App              *app.App
	GameRegistry     *placement.Registry
	AccountRegistry  *placement.Registry
	MaxSessions      int
	mu               sync.Mutex
	users            map[int64]*accountEntry
	disconnectQueues [DisconnectQueueCount]*persistence.Queue
	stopping         bool
}

func (connectionManager *Manager) Init(context.Context) error {
	if connectionManager.MaxSessions < 1 {
		return errors.New("connection: max sessions must be positive")
	}
	connectionManager.users = make(map[int64]*accountEntry)
	// 任一队列都能接下全部预留名额，即使所有账号恰好分到同一队列。
	options := persistence.Options{
		Capacity: connectionManager.MaxSessions,
		MaxBytes: int64(connectionManager.MaxSessions) * 1024,
		Timeout:  8 * time.Second,
	}
	for i := range connectionManager.disconnectQueues {
		queue, err := persistence.NewQueue(options)
		if err != nil {
			for _, started := range connectionManager.disconnectQueues[:i] {
				started.BeginClose()
			}
			return err
		}
		connectionManager.disconnectQueues[i] = queue
	}
	return nil
}

func (connectionManager *Manager) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	var drained [len(connectionManager.disconnectQueues)]<-chan struct{}
	connectionManager.mu.Lock()
	connectionManager.stopping = true
	for i, queue := range connectionManager.disconnectQueues {
		drained[i] = queue.BeginClose()
	}
	connectionManager.mu.Unlock()

	for _, done := range drained {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (connectionManager *Manager) disconnectQueueFor(userID string) *persistence.Queue {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(userID))
	return connectionManager.disconnectQueues[hash.Sum64()%uint64(len(connectionManager.disconnectQueues))]
}

// 调用方持有管理锁，在线名额转入任一队列期间不会被另一笔登录抢占。
func (connectionManager *Manager) usedCloseSlotsLocked() int {
	used := len(connectionManager.users)
	for _, queue := range connectionManager.disconnectQueues {
		used += queue.Stats().Pending
	}
	return used
}

// ReserveConnection 为每个在线账号预留一次下线通知；同网关替换沿用原名额。
func (connectionManager *Manager) ReserveConnection(id access.Identity) error {
	accountID, err := clusterlink.ParseAccountID(id.UserID)
	if err != nil {
		return err
	}
	connectionManager.mu.Lock()
	defer connectionManager.mu.Unlock()
	if connectionManager.stopping {
		return host.ErrNotReady
	}
	slot := connectionManager.users[accountID]
	if slot == nil {
		if connectionManager.usedCloseSlotsLocked() >= connectionManager.MaxSessions {
			return access.ErrCapacity
		}
		slot = &accountEntry{turn: make(chan struct{}, 1)}
		connectionManager.users[accountID] = slot
	}
	slot.current = &connection{
		slot: slot,
		binding: clusterlink.Binding{
			Identity: id,
			Token:    rand.Text(),
		},
	}
	return nil
}
func (connectionManager *Manager) OnDisconnected(id access.Identity) {
	accountID, err := clusterlink.ParseAccountID(id.UserID)
	if err != nil {
		return
	}
	connectionManager.mu.Lock()
	defer connectionManager.mu.Unlock()
	slot := connectionManager.users[accountID]
	if slot == nil || slot.current.binding.Identity != id {
		return
	}
	batch := observability.RequestContext(context.Background())
	_, err = connectionManager.disconnectQueueFor(id.UserID).SubmitContext(batch, func(ctx context.Context) error {
		closeErr := slot.inTurn(ctx, func() error {
			return slot.notifyClosed(ctx, notifyBackend(connectionManager.App.RPCQueuedSessionClosed))
		})
		if closeErr != nil {
			observability.Logger(ctx).Error("下线通知未确认，保留后端归属", "component", "connection", "stage", "disconnect", "kind", "rpc_unconfirmed")
		}
		return closeErr
	}, 512)
	// 在同一把锁内将在线名额转给队列。Pending 包含正在执行的任务，执行完才让出容量。
	delete(connectionManager.users, accountID)
	if err != nil {
		observability.Logger(batch).Error("下线通知未入队，保留后端归属", "component", "connection", "stage", "enqueue", "kind", "queue_rejected")
	}
}

func (slot *accountEntry) inTurn(ctx context.Context, run func() error) error {
	select {
	case slot.turn <- struct{}{}:
		defer func() { <-slot.turn }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return run()
}

func (connectionManager *Manager) current(ctx context.Context) (*connection, error) {
	id, err := access.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	client := connectionManager.findConnection(id)
	if client == nil {
		return nil, access.ErrUnauthenticated
	}
	return client, nil
}

func (connectionManager *Manager) findConnection(id access.Identity) *connection {
	accountID, err := clusterlink.ParseAccountID(id.UserID)
	if err != nil {
		return nil
	}
	connectionManager.mu.Lock()
	defer connectionManager.mu.Unlock()
	if slot := connectionManager.users[accountID]; slot != nil && slot.current.binding.Identity == id {
		return slot.current
	}
	return nil
}

// 只读取创建连接时确定的身份和 token。推送不能等账号锁，网关可能正持锁等 game 回包。
func (connectionManager *Manager) matchesConnection(binding clusterlink.Binding) bool {
	client := connectionManager.findConnection(binding.Identity)
	return client != nil && binding.Token != "" && client.binding.Token == binding.Token
}

// withReadyConnection 串行处理同一条已登录连接的请求，下线通知等当前转发结束后再发送。
func (connectionManager *Manager) withReadyConnection(ctx context.Context, handle func(*connection) error) error {
	client, err := connectionManager.current(ctx)
	if err != nil {
		return err
	}
	if client.login.Load() != loginReady {
		return access.ErrUnauthenticated
	}
	return client.slot.inTurn(ctx, func() error {
		if connectionManager.findConnection(client.binding.Identity) != client {
			return access.ErrUnauthenticated
		}
		return handle(client)
	})
}

// Bind 在登录响应前让原后端接受新连接；只换连接，不更换后端或重新加载业务状态。
func (connectionManager *Manager) Bind(ctx context.Context) error {
	client, err := connectionManager.current(ctx)
	if err != nil {
		return err
	}
	// 每条连接只接线一次。重复登录不能因为排队超时而关闭原先有效的连接。
	if !client.login.CompareAndSwap(loginReserved, loginInProgress) {
		return access.ErrDuplicate
	}
	err = client.slot.inTurn(ctx, func() error {
		if connectionManager.findConnection(client.binding.Identity) != client {
			return access.ErrUnauthenticated
		}
		return connectionManager.bindBackends(ctx, client)
	})
	if err != nil {
		_ = connectionManager.App.CloseClientConnection(ctx)
	}
	return err
}

func (connectionManager *Manager) bindBackends(ctx context.Context, entry *connection) error {
	entry.binding.Gateway = connectionManager.App.ServerID()
	// 上一条连接可能在换绑回包前被顶掉。先处理未确认的目标，再允许本次发送。
	if err := entry.slot.closePending(ctx, notifyBackend(connectionManager.App.RPCSessionClosed)); err != nil {
		return err
	}
	previousAccount, err := connectionManager.attachAccount(ctx, entry)
	if err != nil {
		return err
	}
	previousGame, err := connectionManager.attach(ctx, entry, false)
	if err != nil {
		return err
	}
	if err = connectionManager.directory(ctx, entry, clusterlink.Check, new(pb.CurrentReq), new(pb.ListRolesRep)); err != nil {
		return err
	}
	if err = connectionManager.closePrevious(ctx, previousAccount); err != nil {
		return err
	}
	if previousGame != previousAccount {
		if err = connectionManager.closePrevious(ctx, previousGame); err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if connectionManager.findConnection(entry.binding.Identity) != entry {
		return access.ErrUnauthenticated
	}
	entry.login.Store(loginReady)
	return nil
}
