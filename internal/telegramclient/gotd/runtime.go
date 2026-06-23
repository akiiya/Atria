package gotd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/updates"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/mtproto"
	"github.com/user/atria/internal/security"
	"github.com/user/atria/internal/telegramclient"

	"gorm.io/gorm"
)

// AccountRuntime 管理单个 Telegram 账号的运行时生命周期。
// 持有一个长-lived telegram.Client、updates.Manager 和 execution queue。
type AccountRuntime struct {
	accountID uint
	state     telegramclient.RuntimeState
	cancel    context.CancelFunc
	logger    *slog.Logger
	executor  *RuntimeExecutor // 串行执行队列

	mu        sync.Mutex
	lastSync  *time.Time
	lastEvent *time.Time
	lastError string
}

// GetState 返回当前状态。
func (r *AccountRuntime) GetState() telegramclient.RuntimeStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return telegramclient.RuntimeStatus{
		AccountID:   r.accountID,
		State:       r.state,
		LastSyncAt:  r.lastSync,
		LastEventAt: r.lastEvent,
		LastError:   r.lastError,
	}
}

// setState 更新状态。
func (r *AccountRuntime) setState(state telegramclient.RuntimeState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = state
}

// setSynced 更新最后同步时间。
func (r *AccountRuntime) setSynced() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.lastSync = &now
}

// setError 更新最后错误。
func (r *AccountRuntime) setError(err string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastError = err
}

// clearError 清除错误。
func (r *AccountRuntime) clearError() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastError = ""
}

// setEvent 更新最后事件时间。
func (r *AccountRuntime) setEvent() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.lastEvent = &now
}

// RuntimeManagerImpl 实现 telegramclient.RuntimeManager 接口。
// 管理多个 AccountRuntime，每个 active Telegram account 一个。
type RuntimeManagerImpl struct {
	runtimes map[uint]*AccountRuntime
	mu       sync.RWMutex
	db       *gorm.DB
	key      []byte
	bus      *telegramclient.EventBus
	logger   *slog.Logger
	gate     *AccountGate // per-account 执行锁

	// dialFunc 用于代理
	dialFunc dcs.DialFunc
}

// NewRuntimeManager 创建 RuntimeManagerImpl。
func NewRuntimeManager(db *gorm.DB, key []byte, bus *telegramclient.EventBus, logger *slog.Logger) *RuntimeManagerImpl {
	return &RuntimeManagerImpl{
		runtimes: make(map[uint]*AccountRuntime),
		db:       db,
		key:      key,
		bus:      bus,
		logger:   logger,
		gate:     NewAccountGate(),
	}
}

// SetDialer 设置代理拨号函数。
func (m *RuntimeManagerImpl) SetDialer(fn dcs.DialFunc) {
	m.dialFunc = fn
}

// SetGate 设置 per-account 执行锁。
// 必须在 StartAccount 之前调用，且应与 Adapter 共享同一个 gate。
func (m *RuntimeManagerImpl) SetGate(gate *AccountGate) {
	m.gate = gate
}

// StartAccount 启动指定账号的运行时连接。
// 如果已启动，返回 nil（幂等）。
func (m *RuntimeManagerImpl) StartAccount(accountID uint) error {
	m.mu.Lock()
	if rt, ok := m.runtimes[accountID]; ok {
		state := rt.GetState().State
		if state == telegramclient.RuntimeStateLive ||
			state == telegramclient.RuntimeStateConnecting ||
			state == telegramclient.RuntimeStateSyncing {
			m.mu.Unlock()
			return nil // 已启动
		}
		// 如果是 stopped/degraded/offline，清理后重新启动
		rt.cancel()
		delete(m.runtimes, accountID)
	}
	m.mu.Unlock()

	// 查询账号信息
	var account model.TelegramAccount
	err := m.db.Preload("Session").Where("id = ? AND status = ?", accountID, model.TelegramAccountStatusActive).
		First(&account).Error
	if err != nil {
		return fmt.Errorf("查询账号失败: %w", err)
	}
	if account.Session == nil {
		return fmt.Errorf("账号 %d 没有 session", accountID)
	}

	// 查询 API 凭据
	var cred model.APICredential
	if err := m.db.First(&cred, account.APICredentialID).Error; err != nil {
		return fmt.Errorf("查询 API 凭据失败: %w", err)
	}

	apiHash, err := security.DecryptAPIHash(m.key, cred.EncryptedAPIHash)
	if err != nil {
		return fmt.Errorf("解密 API Hash 失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	rt := &AccountRuntime{
		accountID: accountID,
		state:     telegramclient.RuntimeStateConnecting,
		cancel:    cancel,
		logger:    m.logger,
		executor:  NewRuntimeExecutor(accountID, 64, m.logger),
	}

	m.mu.Lock()
	m.runtimes[accountID] = rt
	m.mu.Unlock()

	// 启动 runtime goroutine
	go m.runAccount(ctx, rt, int(cred.APIID), apiHash, account.Session.SessionFilePath, account.UserID)

	m.logger.Info("AccountRuntime 启动", "account_id", accountID)
	return nil
}

// runAccount 运行单个账号的 gotd client + updates.Manager + execution queue。
// 此函数阻塞直到 context 取消或发生致命错误。
// 不再长期持有 AccountGate，REST 请求通过 executor 使用 runtime client。
func (m *RuntimeManagerImpl) runAccount(ctx context.Context, rt *AccountRuntime, apiID int, apiHash string, sessionFilePath string, userID int64) {
	defer func() {
		rt.setState(telegramclient.RuntimeStateStopped)
		rt.executor.Close() // 关闭 executor，排空等待中的请求
		m.logger.Info("AccountRuntime 停止", "account_id", rt.accountID)
	}()

	// 创建 updates handler，每次处理 update 时更新 runtime 的 lastEvent
	handler := NewUpdateHandler(rt.accountID, m.db, m.key, m.bus, m.logger, rt.setEvent)

	// 创建 updates.Manager
	updatesMgr := updates.New(updates.Config{
		Handler:      handler,
		Storage:      NewStateStore(m.db, m.logger),
		AccessHasher: NewHashStore(m.db, m.key, m.logger),
		Logger:       nil, // gotd 使用 zap，我们传 nil 使用默认 nop
	})

	// 创建 session storage
	storage := mtproto.NewFileBackedSessionStorage(m.key, sessionFilePath)

	// 创建 telegram.Client，设置 UpdateHandler
	opts := telegram.Options{
		SessionStorage: storage,
		UpdateHandler:  updatesMgr,
	}
	if m.dialFunc != nil {
		opts.Resolver = dcs.Plain(dcs.PlainOptions{
			Dial: m.dialFunc,
		})
	}

	client := telegram.NewClient(apiID, apiHash, opts)

	// 运行 client
	rt.setState(telegramclient.RuntimeStateConnecting)
	rt.clearError()

	err := client.Run(ctx, func(runCtx context.Context) error {
		// 连接成功
		rt.setState(telegramclient.RuntimeStateSyncing)
		m.bus.Publish(rt.accountID, telegramclient.UpdateEvent{
			EventID:   fmt.Sprintf("conn_%d_%d", rt.accountID, time.Now().UnixNano()),
			AccountID: rt.accountID,
			Type:      telegramclient.EventAccountConnected,
			CreatedAt: time.Now(),
		})

		// 启动 executor 消费 goroutine
		// executor 与 updates.Manager 共用同一个 client.API()
		// gotd *tg.Client 通过 connection manager 路由，支持并发调用
		go rt.executor.Run(runCtx, client.API())

		// 启动 updates.Manager
		// 这会阻塞，处理 state sync 和 getDifference
		rt.setState(telegramclient.RuntimeStateLive)
		rt.setSynced()
		m.bus.Publish(rt.accountID, telegramclient.UpdateEvent{
			EventID:   fmt.Sprintf("sync_%d_%d", rt.accountID, time.Now().UnixNano()),
			AccountID: rt.accountID,
			Type:      telegramclient.EventSyncDone,
			CreatedAt: time.Now(),
		})

		return updatesMgr.Run(runCtx, client.API(), userID, updates.AuthOptions{
			Forget: false,
			OnStart: func(startCtx context.Context) {
				m.logger.Info("updates.Manager 启动完成", "account_id", rt.accountID)
			},
		})
	})

	if err != nil {
		if ctx.Err() != nil {
			// 正常关闭
			return
		}
		rt.setState(telegramclient.RuntimeStateDegraded)
		rt.setError(err.Error())
		m.logger.Error("AccountRuntime 运行失败",
			"account_id", rt.accountID,
			"error", err,
		)
		m.bus.Publish(rt.accountID, telegramclient.UpdateEvent{
			EventID:   fmt.Sprintf("err_%d_%d", rt.accountID, time.Now().UnixNano()),
			AccountID: rt.accountID,
			Type:      telegramclient.EventAccountDisconnected,
			CreatedAt: time.Now(),
		})
	}
}

// StopAccount 停止指定账号的运行时连接。
func (m *RuntimeManagerImpl) StopAccount(accountID uint) error {
	m.mu.Lock()
	rt, ok := m.runtimes[accountID]
	if !ok {
		m.mu.Unlock()
		return nil // 未启动
	}
	delete(m.runtimes, accountID)
	m.mu.Unlock()

	rt.cancel()
	m.logger.Info("AccountRuntime 停止请求", "account_id", accountID)
	return nil
}

// Status 获取指定账号的运行时状态。
func (m *RuntimeManagerImpl) Status(accountID uint) telegramclient.RuntimeStatus {
	m.mu.RLock()
	rt, ok := m.runtimes[accountID]
	m.mu.RUnlock()

	if !ok {
		return telegramclient.RuntimeStatus{
			AccountID: accountID,
			State:     telegramclient.RuntimeStateStopped,
		}
	}
	return rt.GetState()
}

// Subscribe 订阅指定账号的更新事件。
func (m *RuntimeManagerImpl) Subscribe(accountID uint, sink telegramclient.UpdateSink) (telegramclient.Subscription, error) {
	return m.bus.Subscribe(accountID, sink)
}

// GetExecutor 获取指定账号的 execution queue。
// 只有 runtime 处于 live/syncing 状态时返回 executor。
// connecting 状态时 executor.Run() 尚未启动，不应返回，否则请求会永久阻塞。
// stopped/degraded/offline/connecting 返回 nil，触发 temporary client fallback。
func (m *RuntimeManagerImpl) GetExecutor(accountID uint) *RuntimeExecutor {
	m.mu.RLock()
	rt, ok := m.runtimes[accountID]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	state := rt.GetState().State
	if state == telegramclient.RuntimeStateLive ||
		state == telegramclient.RuntimeStateSyncing {
		return rt.executor
	}
	return nil
}

// StopAll 停止所有运行时。
func (m *RuntimeManagerImpl) StopAll() {
	m.mu.Lock()
	runtimes := make(map[uint]*AccountRuntime)
	for k, v := range m.runtimes {
		runtimes[k] = v
	}
	m.runtimes = make(map[uint]*AccountRuntime)
	m.mu.Unlock()

	for _, rt := range runtimes {
		rt.cancel()
	}
	m.logger.Info("所有 AccountRuntime 已停止")
}

// ReloadDialer 从数据库重新读取代理配置并更新 dialFunc。
// 返回新 dialer 是否可用（api_proxy 类型返回 false）。
func (m *RuntimeManagerImpl) ReloadDialer(db *gorm.DB, key []byte) (available bool, err error) {
	// 需要导入 BuildProxyDialerFromDB，但它在 server 包中
	// 这里直接读取配置重建 dialer
	return m.rebuildDialer(db, key)
}

// rebuildDialer 从数据库重新读取代理配置并更新 m.dialFunc。
func (m *RuntimeManagerImpl) rebuildDialer(db *gorm.DB, key []byte) (bool, error) {
	// 读取代理配置
	var settings []model.SystemSetting
	db.Where("key IN ?", []string{
		"proxy_enabled", "proxy_type", "proxy_host", "proxy_port",
		"proxy_username", "proxy_timeout", "proxy_password",
	}).Find(&settings)

	settingMap := make(map[string]string, len(settings))
	for _, st := range settings {
		settingMap[st.Key] = st.Value
	}

	// 检查代理是否启用
	if settingMap["proxy_enabled"] != "true" && settingMap["proxy_type"] == "none" {
		m.SetDialer(nil)
		return true, nil
	}

	proxyType := settingMap["proxy_type"]
	if proxyType == "none" || proxyType == "" {
		m.SetDialer(nil)
		return true, nil
	}

	// api_proxy 已移除，旧数据库中可能残留此配置
	if proxyType == "api_proxy" {
		m.SetDialer(nil)
		return false, fmt.Errorf("API Proxy 已移除，不适用于 MTProto 连接，请在设置中重新选择 SOCKS5 或 HTTPS CONNECT 代理")
	}

	host := settingMap["proxy_host"]
	portStr := settingMap["proxy_port"]
	if host == "" || portStr == "" {
		return false, fmt.Errorf("代理配置不完整，请检查代理类型、主机和端口")
	}

	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if port < 1 || port > 65535 {
		return false, fmt.Errorf("代理端口无效: %s", portStr)
	}

	timeout := 30 * time.Second
	if t := settingMap["proxy_timeout"]; t != "" {
		secs := 0
		if _, err := fmt.Sscanf(t, "%d", &secs); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}

	username := settingMap["proxy_username"]

	// 读取代理密码
	password := ""
	if pwdValue, ok := settingMap["proxy_password"]; ok && pwdValue != "" {
		decrypted, err := decryptProxyPassword(key, pwdValue)
		if err != nil {
			m.logger.Error("解密代理密码失败", "error", err)
			return false, fmt.Errorf("代理密码配置错误，请重新配置代理")
		}
		password = decrypted
	}

	// 构建 dialer（使用 network 包的工厂函数）
	dialer := buildDialerFromConfig(proxyType, host, port, username, password, timeout)
	m.SetDialer(dialer)
	return true, nil
}

// OnProxySettingsChanged 代理配置变更时调用。
// 1. 重新读取代理配置，更新 dialFunc
// 2. 停止所有运行时（它们会用旧 dialer）
// 3. 返回 dialer 是否可用于 MTProto
func (m *RuntimeManagerImpl) OnProxySettingsChanged(db *gorm.DB, key []byte) (available bool, err error) {
	// 重建 dialer
	available, err = m.rebuildDialer(db, key)
	if err != nil {
		m.logger.Warn("代理配置变更：dialer 不可用", "error", err)
	}

	// 停止所有运行时，让它们用新配置重新启动
	m.StopAll()

	m.logger.Info("代理配置变更：运行时已停止，等待重新启动",
		"dialer_available", available,
	)

	return available, err
}

// 确保 RuntimeManagerImpl 实现 telegramclient.RuntimeManager。
var _ telegramclient.RuntimeManager = (*RuntimeManagerImpl)(nil)

// decryptProxyPassword 解密代理密码。
func decryptProxyPassword(key []byte, encrypted string) (string, error) {
	// 使用与 proxy_helper.go 相同的加密方式
	// crypto.DecryptString(key, ciphertext, aad)
	// AAD: "atria:proxy:v1"
	decrypted, err := cryptoDecryptString(key, encrypted, []byte("atria:proxy:v1"))
	if err != nil {
		return "", err
	}
	return decrypted, nil
}

// cryptoDecryptString 是 crypto.DecryptString 的引用。
// 为了避免循环依赖，使用函数变量注入。
var cryptoDecryptString = func(key []byte, ciphertext string, aad []byte) (string, error) {
	return "", fmt.Errorf("cryptoDecryptString 未注入")
}

// InjectCryptoFunctions 注入加密函数，避免循环依赖。
func InjectCryptoFunctions(
	decryptFn func(key []byte, ciphertext string, aad []byte) (string, error),
) {
	cryptoDecryptString = decryptFn
}

// buildDialerFromConfig 从代理配置构建 dialer。
func buildDialerFromConfig(proxyType, host string, port int, username, password string, timeout time.Duration) dcs.DialFunc {
	// 使用 network 包的工厂函数
	dialer := newDialerFromConfig(proxyType, host, port, username, password, timeout)
	if dialer == nil {
		return nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}
}

// newDialerFromConfig 是 network.NewDialer 的引用。
// 为了避免循环依赖，使用函数变量注入。
var newDialerFromConfig = func(proxyType, host string, port int, username, password string, timeout time.Duration) DialerInterface {
	return nil
}

// DialerInterface 是 network.Dialer 的本地接口，供 server 包注入使用。
type DialerInterface interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// InjectNetworkFunctions 注入网络函数，避免循环依赖。
func InjectNetworkFunctions(
	newDialerFn func(proxyType, host string, port int, username, password string, timeout time.Duration) DialerInterface,
) {
	newDialerFromConfig = newDialerFn
}
