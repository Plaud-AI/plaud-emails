package snowflake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/etcd"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/svc"

	"github.com/bwmarrin/snowflake"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// GeneratorCreater Snowflake 生成器创建器
type GeneratorCreater struct {
	svc.BaseService
	cfg               *config.SnowflakeConfig
	registry          *etcd.ServiceRegistry
	claimKey          string
	nodeHearbetCancel context.CancelFunc
}

// NewCreater 创建空工厂
func NewCreater(cfg *config.SnowflakeConfig, registry *etcd.ServiceRegistry) *GeneratorCreater {
	cloneCfg := *cfg
	return &GeneratorCreater{cfg: &cloneCfg, registry: registry}
}

// Init 初始化 snowflake 生成器
func (f *GeneratorCreater) Init(ctx context.Context) error {
	if f.IsInited() {
		return nil
	}

	if f.cfg == nil {
		return errors.New("snowflake config is nil")
	}
	if !f.cfg.UseEtcd {
		logger.Infof("snowflake: use etcd is false, will use local node_id")
		return nil
	} else if f.registry == nil {
		return errors.New("snowflake: etcd registry is nil")
	}

	start := f.cfg.EtcdNodIDRange[0]
	end := f.cfg.EtcdNodIDRange[1]
	if start < 0 || end < start {
		return fmt.Errorf("invalid etcd_node_id_range: [%d, %d]", start, end)
	}

	var (
		client    *clientv3.Client = f.registry.GetClient()
		serverID  string           = f.registry.GetServerID()
		keyPrefix                  = buildPrefix(f.registry.GetServicePrefix()) + "/snowflake/nodes"
	)

	// 如果不为空则在keyPrefix后面拼接，构建指定服务的NodeID的区间Key
	if f.cfg.EtcdSuffix != "" {
		keyPrefix = keyPrefix + "/" + strings.TrimPrefix(f.cfg.EtcdSuffix, "/")
	}

	// soft TTL and heartbeat settings
	const (
		defaultGraceDuration = 60 * time.Minute
		heartbeatInterval    = 60 * time.Second
	)

	// resolve grace duration from env var SNOWFLAKE_STALE_SECOND (seconds)
	staleDuration := defaultGraceDuration
	if v := os.Getenv("SNOWFLAKE_STALE_SECOND"); v != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			staleDuration = time.Duration(n) * time.Second
		} else if err != nil {
			log.Printf("snowflake: invalid SNOWFLAKE_STALE_SECOND=%q: %v, using default %s", v, err, defaultGraceDuration)
		}
	}

	// pre-cleanup: scan and atomically delete stale node claims before allocation
	for id := start; id <= end; id++ {
		key := fmt.Sprintf("%s/%d", keyPrefix, id)
		getResp, err := client.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get etcd key: %w", err)
		}
		if len(getResp.Kvs) == 0 {
			continue
		}
		kv := getResp.Kvs[0]
		var nc nodeClaim
		if err := json.Unmarshal(kv.Value, &nc); err != nil {
			log.Printf("snowflake: malformed claim %s:%s", key, string(kv.Value))
			continue
		}
		if nc.HeartbeatAt < time.Now().Add(-staleDuration).UnixMilli() {
			// atomic delete only if key unchanged since read
			txn, err := client.Txn(ctx).If(
				clientv3.Compare(clientv3.ModRevision(key), "=", kv.ModRevision),
			).Then(
				clientv3.OpDelete(key),
			).Commit()
			if err != nil {
				log.Printf("snowflake: cleanup txn failed for %s: %v", key, err)
				continue
			}
			if txn.Succeeded {
				log.Printf("snowflake: deleted stale node claim %s (owner=%s, last_heartbeat_at=%s)", key, nc.Owner, time.UnixMilli(nc.HeartbeatAt).Format(time.RFC3339))
			}
		}
	}
	var (
		allocatedID int64 = -1
		claimKey    string
		staleClaims []nodeClaim
	)

	for id := start; id <= end; id++ {
		key := fmt.Sprintf("%s/%d", keyPrefix, id)
		getResp, err := client.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get etcd key: %w", err)
		}

		nowMs := time.Now().UnixMilli()
		if len(getResp.Kvs) > 0 {
			var nc nodeClaim
			var kv = getResp.Kvs[0]
			if err := json.Unmarshal(kv.Value, &nc); err != nil {
				logger.Errorf("snowflake: malformed claim %s:%s", key, string(kv.Value))
			} else {
				// 记录可能过期的节点
				if nc.HeartbeatAt < nowMs-int64(staleDuration/time.Millisecond) {
					nc.NodeID = string(kv.Key)
					staleClaims = append(staleClaims, nc)
				}
			}
			continue
		}

		myClaim := nodeClaim{Owner: serverID, HeartbeatAt: nowMs}
		valBytes, _ := json.Marshal(myClaim)
		txn, err := client.Txn(ctx).If(
			clientv3.Compare(clientv3.Version(key), "=", 0),
		).Then(
			clientv3.OpPut(key, string(valBytes)),
		).Commit()
		if err != nil {
			return fmt.Errorf("failed to commit etcd txn: %w", err)
		}
		if txn.Succeeded {
			allocatedID = id
			claimKey = key
			break
		}
	}

	// 如果有过期的节点，记录错误日志发出告警。
	// TODO: 此时无法判断持有NodeID的节点是否正常工作，保险起见需要人工介入决定是否删除对应的key
	for _, expiredClaim := range staleClaims {
		logger.Errorf("snowflake: stale claim key:%s, owner:%s, last_heartbeat_at:%s", expiredClaim.NodeID, expiredClaim.Owner, time.UnixMilli(expiredClaim.HeartbeatAt).Format(time.RFC3339))
	}

	if allocatedID < 0 {
		return fmt.Errorf("no available node_id in range [%d, %d]", start, end)
	}

	oldNodeID := f.cfg.NodeID
	f.cfg.NodeID = allocatedID
	f.claimKey = claimKey
	heartbetCtx, nodeHearbetCancel := context.WithCancel(context.Background())
	f.nodeHearbetCancel = nodeHearbetCancel

	// start heartbeat to refresh claim periodically
	go f.heartbeat(heartbetCtx, heartbeatInterval)

	logger.Infof("snowflake: allocated node_id %d via etcd (init), old node_id %d", allocatedID, oldNodeID)
	f.SetInited(true)
	return nil
}

func (f *GeneratorCreater) Stop(ctx context.Context) error {
	if f.IsStopped() {
		return nil
	}
	defer f.SetStopped(true)

	if f.nodeHearbetCancel != nil {
		f.nodeHearbetCancel()
		time.Sleep(100 * time.Millisecond)
	}
	if f.registry == nil {
		return nil
	}

	client := f.registry.GetClient()
	if client != nil && f.claimKey != "" {
		delCtx, delCancel := context.WithTimeout(ctx, 5*time.Second)
		defer delCancel()

		getResp, err := client.Get(delCtx, f.claimKey)
		if err != nil {
			logger.Warnf("snowflake: get node claim %s before delete failed: %v", f.claimKey, err)
			return nil
		}
		if len(getResp.Kvs) == 0 {
			logger.Infof("snowflake: node claim %s already removed", f.claimKey)
			return nil
		}
		var claim nodeClaim
		if err := json.Unmarshal(getResp.Kvs[0].Value, &claim); err != nil {
			logger.Warnf("snowflake: malformed node claim %s before delete: %v", f.claimKey, err)
			return nil
		}
		serverID := f.registry.GetServerID()
		if claim.Owner != serverID {
			logger.Warnf("snowflake: skip deleting node claim %s due to owner mismatch (expected=%s, actual=%s)", f.claimKey, serverID, claim.Owner)
			return nil
		}

		if _, err := client.Delete(delCtx, f.claimKey); err != nil {
			logger.Warnf("snowflake: delete node claim %s failed: %v", f.claimKey, err)
		} else {
			logger.Infof("snowflake: deleted node claim %s", f.claimKey)
		}
	}
	return nil
}

// Create 根据 SnowflakeConfig 创建并返回生成器，建议每个表/业务创建一个生成器
func (f *GeneratorCreater) Create() (*Generator, error) {
	gen, err := NewFromConfig(f.cfg)
	if err != nil {
		return nil, err
	}
	return gen, nil
}

func buildPrefix(prefix string) string {
	if prefix == "" {
		return ""
	}
	if prefix[0] != '/' {
		return "/" + prefix
	}
	return prefix
}

// Generator wraps a snowflake.Node with optional custom epoch.
type Generator struct {
	node   *snowflake.Node
	nodeID int64
}

var (
	initEpochOnce sync.Once
)

// NewFromConfig initializes a Generator from SnowflakeConfig.
// cfg must be non-nil and should be parsed beforehand so defaults are applied.
func NewFromConfig(cfg *config.SnowflakeConfig) (*Generator, error) {
	if cfg == nil {
		return nil, errors.New("snowflake config is nil")
	}
	// Epoch must be set by cfg.Parse(); still guard parse errors here
	if t, err := time.Parse(time.RFC3339, cfg.Epoch); err == nil {
		// 只初始化一次
		initEpochOnce.Do(func() {
			snowflake.Epoch = t.UnixNano() / int64(time.Millisecond)
		})
	} else {
		logger.Warnf("invalid snowflake epoch: %v", err)
		return nil, err
	}

	n, err := snowflake.NewNode(cfg.NodeID)
	if err != nil {
		return nil, err
	}
	return &Generator{node: n, nodeID: cfg.NodeID}, nil
}

func (p *Generator) NextID() int64 {
	return p.node.Generate().Int64()
}

type nodeClaim struct {
	NodeID      string `json:"node_id,omitempty"`
	Owner       string `json:"owner"`
	HeartbeatAt int64  `json:"heartbeat_at"`
}

func (f *GeneratorCreater) heartbeat(ctx context.Context, interval time.Duration) {
	if f.claimKey == "" {
		return
	}
	logger.Infof("snowflake: start heartbeat for node_id key %s", f.claimKey)
	ticker := time.NewTicker(interval)
	defer func() {
		ticker.Stop()
		logger.Infof("snowflake: heartbeat for node_id key %s stopped", f.claimKey)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.keepAlive(ctx)
		}
	}
}

func (f *GeneratorCreater) keepAlive(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := f.registry.GetClient()
	if client == nil {
		logger.Warnf("snowflake: heartbeat get client failed")
		return
	}
	getResp, err := client.Get(ctx, f.claimKey)
	if err != nil || len(getResp.Kvs) == 0 {
		logger.Warnf("snowflake: heartbeat get failed or key missing: %v", err)
		return
	}
	kv := getResp.Kvs[0]
	var claim nodeClaim
	if err := json.Unmarshal(kv.Value, &claim); err != nil {
		logger.Warnf("snowflake: malformed claim during heartbeat: %v", err)
		return
	}
	if claim.Owner != f.registry.GetServerID() {
		logger.Errorf("snowflake: lost ownership of node_id key %s", f.claimKey)
		return
	}
	claim.HeartbeatAt = time.Now().UnixMilli()
	valBytes, _ := json.Marshal(claim)
	txn, err := client.Txn(ctx).If(clientv3.Compare(clientv3.ModRevision(f.claimKey), "=", kv.ModRevision)).Then(clientv3.OpPut(f.claimKey, string(valBytes))).Commit()
	if err != nil {
		logger.Warnf("snowflake: heartbeat txn failed: %v", err)
		return
	}
	if !txn.Succeeded {
		logger.Errorf("snowflake: heartbeat CAS failed, ownership changed for %s", f.claimKey)
	}
}
