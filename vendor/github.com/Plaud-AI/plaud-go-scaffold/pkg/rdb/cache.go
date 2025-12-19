package rdb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
)

// 重复的命名空间保护，避免业务误用相同的 key 命名空间
var dupCacheNamespace sync.Map

// CacheConfig 描述某一类缓存的配置（命名空间、TTL、是否访问自动续期等）。
// - namespace: 统一 key 命名空间/分组
// - ttl: 默认过期时间（0 表示不过期）
// - renewTTL : 是否在访问时自动续期（续期到 ttl）
type CacheConfig struct {
	namespace string
	ttl       time.Duration
	renewTTL  bool
}

// NewCacheConfig 基于命名空间创建配置，若命名空间重复会 panic（防止业务误用重复命名空间）
func NewCacheConfig(namespace string, ttl time.Duration, renewTTL bool) *CacheConfig {
	ns := strings.TrimSpace(namespace)
	if _, exist := dupCacheNamespace.LoadOrStore(ns, struct{}{}); exist {
		panic("duplicate cache namespace: " + ns)
	}
	return &CacheConfig{namespace: ns, ttl: ttl, renewTTL: renewTTL}
}

// NewCacheConfigFromParts 通过多段拼接命名空间（以 ':' 连接）
func NewCacheConfigFromParts(ttl time.Duration, renewTTL bool, parts ...string) *CacheConfig {
	if len(parts) == 0 {
		panic("empty cache namespace parts")
	}
	namespace := strings.Join(parts, ":")
	return NewCacheConfig(namespace, ttl, renewTTL)
}

// WithNamespaceSuffix 在现有命名空间后追加一段后缀，返回新配置（不重复校验命名空间）
func (c *CacheConfig) WithNamespaceSuffix(suffix string) *CacheConfig {
	copy := *c
	if strings.TrimSpace(suffix) == "" {
		return &copy
	}
	copy.namespace = c.namespace + ":" + suffix
	return &copy
}

// WithTTL 派生一个仅修改 ttl 的配置（不重复校验命名空间）
func (c *CacheConfig) WithTTL(ttl time.Duration) *CacheConfig {
	copy := *c
	copy.ttl = ttl
	return &copy
}

// WithRenewTTL 派生一个仅修改 renewTTL 的配置（不重复校验命名空间）
func (c *CacheConfig) WithRenewTTL(renewTTL bool) *CacheConfig {
	copy := *c
	copy.renewTTL = renewTTL
	return &copy
}

// Namespace 返回命名空间
func (c *CacheConfig) Namespace() string { return c.namespace }

// TTL 返回默认过期时间
func (c *CacheConfig) TTL() time.Duration { return c.ttl }

// RenewTTL 是否访问自动续期
func (c *CacheConfig) RenewTTL() bool { return c.renewTTL }

// BuildKey 使用命名空间与传入 key 拼接完整 Redis key
func (c *CacheConfig) BuildKey(key string) string {
	if c == nil || c.namespace == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", c.namespace, key)
}

// NewParamKey create new ParamKey with key
func (p *CacheConfig) NewParamKey(key string) *Key {
	return &Key{
		CacheConfig: *p,
		key:         p.BuildKey(key),
	}
}

// NewParamKeys 创建一个包含多个 key 的参数 key
func (p *CacheConfig) NewParamKeys(key string, otherKeys ...string) *Key {
	if len(otherKeys) > 0 {
		keys := make([]string, 0, len(otherKeys)+1)
		keys = append(keys, key)
		keys = append(keys, otherKeys...)
		key = strings.Join(keys, ":")
	}
	return &Key{
		CacheConfig: *p,
		key:         p.BuildKey(key),
	}
}

// NewKeyWithoutNamespace create new ParamKey without namespace
func (p *CacheConfig) NewParamKeyWithoutNamespace(key string) *Key {
	return &Key{
		CacheConfig: *p,
		key:         key,
	}
}

// Key is the cache param with key
type Key struct {
	CacheConfig
	key string
}

// Key implements Param.Key()
func (p *Key) Key() string { return p.key }

// WithTTL new key with ttl
func (p *Key) WithTTL(ttl time.Duration) *Key {
	var k = *p
	k.ttl = ttl
	return &k
}

// GetFromCache 从缓存中获取数据，若缓存不存在，则调用 get 函数获取数据，并回填缓存
func GetFromCache[T any](ctx context.Context, redisClient *Client, key *Key, get func() (getRet *T, getErr error)) (*T, error) {
	if key == nil {
		return nil, fmt.Errorf("key is nil")
	}
	if redisClient == nil && get == nil {
		return nil, fmt.Errorf("redisClient and get are both nil")
	}

	if redisClient == nil {
		return get()
	}

	// 先从缓存中获取
	ret := new(T)
	ok, err := redisClient.GetObject(ctx, key, ret)
	if err != nil {
		return nil, err
	}
	if ok {
		return ret, nil
	}

	if get == nil {
		return nil, nil
	}

	// 再加载数据
	getRet, err := get()
	if err != nil {
		return nil, err
	}

	//回填缓存
	if getRet != nil {
		if setErr := redisClient.SetObject(ctx, key, getRet); setErr != nil {
			logger.Errorf("set object to redis failed", "key", key, "err", setErr)
		}
	}
	return getRet, nil
}
