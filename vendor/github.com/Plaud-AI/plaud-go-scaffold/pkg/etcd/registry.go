package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/svc"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ServiceRegistry ETCD服务注册器
type ServiceRegistry struct {
	svc.BaseService
	client        *clientv3.Client
	leaseID       clientv3.LeaseID
	ttl           int64
	stopChan      chan struct{}
	servicePrefix string
	serverID      string
	serviceAddr   string
	mu            sync.RWMutex
	registrations map[string]*registration
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Addr     string            `json:"addr"`
	Port     int               `json:"port"`
	Metadata map[string]string `json:"metadata,omitempty"`
	UpdateAt int64             `json:"update_at"`
}

func (p *ServiceInfo) GetID() string {
	return fmt.Sprintf("%s:%d", p.Addr, p.Port)
}

// NewServiceRegistry 创建新的服务注册器
func NewServiceRegistry(cfg *config.ETCDConfig, serverID string, exposeAddr string) (*ServiceRegistry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("etcd config is nil")
	}

	var (
		servicePrefix = cfg.ServicePrefix
		endpoints     = cfg.Endpoints
		username      = cfg.Username
		password      = cfg.Password
		ttl           = cfg.TTL
	)

	if ttl <= 0 {
		ttl = 30
	}
	config := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}

	if username != "" && password != "" {
		config.Username = username
		config.Password = password
	}

	client, err := clientv3.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}
	if servicePrefix != "" && !strings.HasPrefix(servicePrefix, "/") {
		servicePrefix = "/" + servicePrefix
	}
	return &ServiceRegistry{client: client, ttl: ttl, stopChan: make(chan struct{}), servicePrefix: servicePrefix, serverID: serverID, serviceAddr: exposeAddr, registrations: make(map[string]*registration)}, nil
}

// Init 初始化服务注册器
func (p *ServiceRegistry) Init(ctx context.Context) (err error) {
	if p.IsInited() {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 创建租约
	lease, err := p.client.Grant(ctx, p.ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}
	p.leaseID = lease.ID
	go p.heartbeat()
	logger.Infof("etcd service registry initialized, serverID: %s", p.serverID)
	p.SetInited(true)
	return
}

// Stop 停止
func (p *ServiceRegistry) Stop(ctx context.Context) (err error) {
	if p.IsStopped() {
		return nil
	}

	logger.Infof("etcd service registry stopping, serverID: %s", p.serverID)
	close(p.stopChan)
	// 稍等50ms, 让心跳和Watch协程退出

	regs := make([]registration, 0, len(p.registrations))
	p.mu.RLock()
	for _, r := range p.registrations {
		regs = append(regs, *r)
	}
	p.mu.RUnlock()

	for _, r := range regs {
		if err := p.Unregister(ctx, r.serviceName, r.info); err != nil {
			logger.Errorf("Failed to unregister service: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)
	if err := p.client.Close(); err != nil {
		logger.Errorf("Failed to close etcd client: %v", err)
		return err
	}
	p.SetStopped(true)
	return nil
}

// Register 注册服务到ETCD
func (p *ServiceRegistry) Register(ctx context.Context, serviceName string, info ServiceInfo) error {
	if serviceName == "" {
		return fmt.Errorf("service name is empty")
	}

	key, err := p.putService(ctx, serviceName, info)
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 记录本地注册，用于 lease 丢失后重注册
	p.mu.Lock()
	p.registrations[key] = &registration{serviceName: serviceName, info: info}
	p.mu.Unlock()

	logger.Infof("%s registered", key)
	return nil
}

// Unregister 从ETCD注销服务
func (p *ServiceRegistry) Unregister(ctx context.Context, serviceName string, info ServiceInfo) error {
	if serviceName == "" {
		return fmt.Errorf("service name is empty")
	}

	key := p.getServiceNodeKey(serviceName, info)
	_, err := p.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to unregister service: %w", err)
	}
	logger.Infof("%s unregistered", key)
	// 从本地注册表中移除
	p.mu.Lock()
	delete(p.registrations, key)
	p.mu.Unlock()
	return nil
}

// heartbeat 心跳保活
func (p *ServiceRegistry) heartbeat() {
	ticker := time.NewTicker(time.Duration(p.ttl/3) * time.Second)
	defer func() {
		ticker.Stop()
		logger.Infof("stop heartbeat")
	}()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			if err := p.keepAlive(); err != nil {
				logger.Errorf("Failed to keep alive: %v", err)
			}
		}
	}
}

// keepAlive 保活
func (p *ServiceRegistry) keepAlive() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := p.client.KeepAliveOnce(ctx, p.leaseID)
	if err != nil {
		// 如果是 lease 不存在（可能因为网络/重启导致的丢失），则重建租约并重注册
		if status.Code(err) == codes.NotFound || strings.Contains(err.Error(), "lease not found") {
			logger.Warnf("lease lost (not found). recreating lease and re-registering services: %v", err)
			if reErr := p.recreateLeaseAndReregister(); reErr != nil {
				return fmt.Errorf("failed to recreate lease and re-register: %w", reErr)
			}
			return nil
		}
		return fmt.Errorf("failed to keep alive: %w", err)
	}
	return nil
}

// GetServices 获取所有服务
func (p *ServiceRegistry) GetServices(ctx context.Context, serviceName string) ([]*ServiceInfo, error) {
	key := p.getServiceKey(serviceName)
	resp, err := p.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	var services []*ServiceInfo
	for _, kv := range resp.Kvs {
		var info ServiceInfo
		if err := json.Unmarshal(kv.Value, &info); err != nil {
			logger.Warnf("Failed to unmarshal service info - key: %s, error: %v", string(kv.Key), err)
			continue
		}
		services = append(services, &info)
	}
	return services, nil
}

// WatchService 监听服务变化
func (p *ServiceRegistry) WatchService(serviceName string, callback func([]*ServiceInfo)) {
	key := p.getServiceKey(serviceName)
	ctx := context.Background()
	watchChan := p.client.Watch(ctx, key, clientv3.WithPrefix())
	go func() {
		defer func() {
			logger.Infof("stop watch service %s", serviceName)
		}()
		for {
			select {
			case <-p.stopChan:
				return
			case resp := <-watchChan:
				if resp.Err() != nil {
					logger.Errorf("Watch error: %v", resp.Err())
					continue
				}
				// 获取最新服务列表
				services, err := p.GetServices(ctx, serviceName)
				if err != nil {
					logger.Errorf("Failed to get services after watch: %v", err)
					continue
				}
				callback(services)
			}
		}
	}()
}

// GetClient 返回底层 etcd client
func (p *ServiceRegistry) GetClient() *clientv3.Client {
	return p.client
}

// GetLeaseID 返回用于注册服务的共享租约ID
func (p *ServiceRegistry) GetLeaseID() clientv3.LeaseID {
	return p.leaseID
}

// GetServicePrefix 返回服务前缀
func (p *ServiceRegistry) GetServicePrefix() string {
	return p.servicePrefix
}

// GetTTL 返回租约TTL（秒）
func (p *ServiceRegistry) GetTTL() int64 {
	return p.ttl
}

func (p *ServiceRegistry) getServiceNodeKey(serviceName string, info ServiceInfo) string {
	serverID := info.GetID()
	if p.serviceAddr != "" {
		//如果服务地址不为空，则使用服务地址作为服务ID
		mockInfo := info
		mockInfo.Addr = p.serviceAddr
		serverID = mockInfo.GetID()
	}
	key := fmt.Sprintf("/services/%s/%s", serviceName, serverID)
	if p.servicePrefix != "" {
		key = p.servicePrefix + key
	}
	return key
}

func (p *ServiceRegistry) getServiceKey(serviceName string) string {
	key := fmt.Sprintf("/services/%s/", serviceName)
	if p.servicePrefix != "" {
		key = p.servicePrefix + key
	}
	return key
}

// GetServerID 返回服务ID
func (p *ServiceRegistry) GetServerID() string {
	return p.serverID
}

// putService 将服务信息以当前 lease 写入 etcd，并返回最终写入的 key
func (p *ServiceRegistry) putService(ctx context.Context, serviceName string, info ServiceInfo) (string, error) {
	if p.serviceAddr != "" {
		info.Addr = p.serviceAddr
	}
	key := p.getServiceNodeKey(serviceName, info)
	info.UpdateAt = time.Now().UnixMilli()
	data, err := json.Marshal(info)
	if err != nil {
		return key, fmt.Errorf("failed to marshal service info: %w", err)
	}
	value := string(data)
	_, err = p.client.Put(ctx, key, value, clientv3.WithLease(p.leaseID))
	if err != nil {
		return key, err
	}
	return key, nil
}

// registration 用于保存本地已注册的服务信息
type registration struct {
	serviceName string
	info        ServiceInfo
}

// recreateLeaseAndReregister 在 lease 丢失时重建并重注册本地服务
func (p *ServiceRegistry) recreateLeaseAndReregister() error {
	// 重建租约
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lease, err := p.client.Grant(ctx, p.ttl)
	if err != nil {
		return fmt.Errorf("failed to create new lease: %w", err)
	}
	p.leaseID = lease.ID

	// 提取当前需要重注册的列表（拷贝快照以避免长时间持锁）
	p.mu.RLock()
	regs := make([]registration, 0, len(p.registrations))
	for _, r := range p.registrations {
		regs = append(regs, *r)
	}
	p.mu.RUnlock()

	// 逐个重注册
	for _, r := range regs {
		ctxPut, cancelPut := context.WithTimeout(context.Background(), 5*time.Second)
		key, pErr := p.putService(ctxPut, r.serviceName, r.info)
		cancelPut()
		if pErr != nil {
			return fmt.Errorf("failed to re-register service key %s: %w", key, pErr)
		}
	}
	logger.Infof("recreated lease and re-registered %d services", len(regs))
	return nil
}

// DialGRPC 通过etcd服务发现为指定serviceName创建gRPC连接，带round_robin与动态更新
func (p *ServiceRegistry) DialGRPC(ctx context.Context, serviceName string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name is empty")
	}
	// Per-dial resolver; no global registration
	builder := &etcdResolverBuilder{registry: p}
	baseOpts := []grpc.DialOption{
		grpc.WithResolvers(builder),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
	}
	allOpts := append(baseOpts, opts...)
	// Support both etcd:///svc and etcd://svc
	target := fmt.Sprintf("etcd:///%s", serviceName)
	cc, err := grpc.NewClient(target, allOpts...)
	if err != nil {
		return nil, err
	}
	// If caller set a deadline on ctx, emulate blocking connect until READY or deadline
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		cc.Connect()
		state := cc.GetState()
		for state != connectivity.Ready {
			if !cc.WaitForStateChange(ctx, state) {
				// ctx done
				return nil, ctx.Err()
			}
			state = cc.GetState()
		}
	}
	return cc, nil
}
