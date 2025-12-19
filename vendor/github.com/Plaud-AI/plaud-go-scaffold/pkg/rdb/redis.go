package rdb

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/go-redis/redis/v8"
)

// Client Redis客户端
type Client struct {
	client *redis.Client
}

// GetObjectByKeyFunc 回源函数类型
type GetObjectByKeyFunc[T any] func(rawKey string, index int) (*T, error)

// NewClient 创建Redis连接管理器
func NewClient(cfg *config.RedisConfig) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("redis config is nil")
	}

	var needTLS = cfg.TLS
	if strings.HasPrefix(cfg.Addr, "rediss://") {
		needTLS = true
		cfg.Addr = strings.TrimPrefix(cfg.Addr, "rediss://")
	} else if strings.HasPrefix(cfg.Addr, "redis://") {
		cfg.Addr = strings.TrimPrefix(cfg.Addr, "redis://")
	}

	if cfg.TLS {
		needTLS = true
	}

	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	if needTLS {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	client := redis.NewClient(opts)

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Errorf("failed to connect to redis: %v", err)
		return nil, err
	}

	logger.Debugf("connected to redis: %s", cfg.Addr)

	return &Client{client: client}, nil
}

// GetClient 获取Redis客户端
func (p *Client) GetClient() *redis.Client { return p.client }

// Close 关闭Redis连接
func (p *Client) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

func (p *Client) RunScript(ctx context.Context, script *Script, keys []string, args ...interface{}) (cmd *redis.Cmd) {
	cmd = p.client.EvalSha(ctx, script.hash, keys, args...)
	err := cmd.Err()
	if err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT ") {
		logger.Warnf("evalsha: %v, try eval script: %s", err, script.hash)
		cmd = p.client.Eval(ctx, script.src, keys, args...)
		return
	}
	return
}

// InitScriptIfAbsent 检查并加载Lua脚本到Redis，如果脚本不存在则初始化
func (p *Client) InitScriptIfAbsent(ctx context.Context, script *Script) (err error) {
	if p.client == nil {
		return redis.ErrClosed
	}
	exists, err := p.client.ScriptExists(ctx, script.hash).Result()
	if err != nil {
		return err
	}
	if len(exists) > 0 && exists[0] {
		return nil
	}
	loadedSha, err := p.client.ScriptLoad(ctx, script.src).Result()
	if err != nil {
		return err
	}
	if loadedSha != script.hash {
		return errors.New("script load failed, loadedSha: " + loadedSha + ", hash: " + script.hash)
	}
	return nil
}

func (p *Client) getLockKey(key string) string { return fmt.Sprintf("lock:%s", key) }

// TryLock 尝试获取锁
func (p *Client) TryLock(ctx context.Context, key string, expire time.Duration) (ok bool, err error) {
	lockKey := p.getLockKey(key)
	lock, err := p.client.SetNX(ctx, lockKey, "", expire).Result()
	if err != nil {
		return
	}
	ok = lock
	return
}

// UnLock 释放锁
func (p *Client) UnLock(ctx context.Context, key string) (err error) {
	lockKey := p.getLockKey(key)
	_, err = p.client.Del(ctx, lockKey).Result()
	return
}

// --- Cache helpers on Client (ParamKey only) ---

func (p *Client) validateParamKey(key *Key) (string, error) {
	if p.client == nil {
		return "", redis.ErrClosed
	}
	if key == nil {
		return "", errors.New("param is nil")
	}
	k := strings.TrimSpace(key.Key())
	if k == "" {
		return "", errors.New("cache key is empty")
	}
	return k, nil
}

// Set 设置二进制值；过期时间使用 key.TTL()
func (p *Client) Set(ctx context.Context, key *Key, value []byte) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	return p.client.Set(ctx, k, value, key.TTL()).Err()
}

// SetString 设置字符串值；过期时间使用 key.TTL()
func (p *Client) SetString(ctx context.Context, key *Key, value string) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	return p.client.Set(ctx, k, value, key.TTL()).Err()
}

// Get 获取二进制数据；若 key.RenewTTL() 且 TTL>0，则使用 GETEX 续期到 TTL
func (p *Client) Get(ctx context.Context, key *Key) ([]byte, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return nil, err
	}
	if key.RenewTTL() && key.TTL() > 0 {
		b, err := p.client.GetEx(ctx, k, key.TTL()).Bytes()
		if err == redis.Nil {
			return nil, nil
		}
		return b, err
	}
	b, err := p.client.Get(ctx, k).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return b, err
}

// GetString 获取字符串；若 key.RenewTTL() 且 TTL>0，则使用 GETEX 续期到 TTL
func (p *Client) GetString(ctx context.Context, key *Key) (string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return "", err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		cmd = p.client.GetEx(ctx, k, key.TTL())
	} else {
		cmd = p.client.Get(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return "", nil
	}
	return cmd.Result()
}

// Del 删除 key
func (p *Client) Del(ctx context.Context, key *Key) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	return p.client.Del(ctx, k).Err()
}

// IncrBy 自增指定步长；若 key.RenewTTL() 且 TTL>0，则在事务中同时续期到 TTL
func (p *Client) IncrBy(ctx context.Context, key *Key, value int64) (int64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	if key.RenewTTL() && key.TTL() > 0 {
		var incrCmd *redis.IntCmd
		_, err := p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			incrCmd = pipe.IncrBy(ctx, k, value)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil {
			return 0, err
		}
		return incrCmd.Val(), nil
	}
	return p.client.IncrBy(ctx, k, value).Result()
}

// Incr 自增 1；若 key.RenewTTL() 且 TTL>0，则在事务中同时续期到 TTL
func (p *Client) Incr(ctx context.Context, key *Key) (int64, error) { return p.IncrBy(ctx, key, 1) }

// GetInt 获取 int 值；若 key.RenewTTL() 且 TTL>0，则使用 GETEX 续期到 TTL
func (p *Client) GetInt(ctx context.Context, key *Key) (int, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		cmd = p.client.GetEx(ctx, k, key.TTL())
	} else {
		cmd = p.client.Get(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return 0, nil
	}
	return cmd.Int()
}

// GetInt64 获取 int64 值；若 key.RenewTTL() 且 TTL>0，则使用 GETEX 续期到 TTL
func (p *Client) GetInt64(ctx context.Context, key *Key) (int64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		cmd = p.client.GetEx(ctx, k, key.TTL())
	} else {
		cmd = p.client.Get(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return 0, nil
	}
	return cmd.Int64()
}

// GetFloat64 获取 float64 值；若 key.RenewTTL() 且 TTL>0，则使用 GETEX 续期到 TTL
func (p *Client) GetFloat64(ctx context.Context, key *Key) (float64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		cmd = p.client.GetEx(ctx, k, key.TTL())
	} else {
		cmd = p.client.Get(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return 0, nil
	}
	return cmd.Float64()
}

// SetObject 使用包级 DefaultCodec 序列化对象并写入；过期时间使用 key.TTL()
func (p *Client) SetObject(ctx context.Context, key *Key, v any) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	data, err := DefaultCodec().Marshal(v)
	if err != nil {
		return err
	}
	return p.client.Set(ctx, k, data, key.TTL()).Err()
}

// GetObject 使用包级 DefaultCodec 读取并反序列化到 out；若 key.RenewTTL() 且 TTL>0，则使用 GETEX 续期到 TTL
// 返回 found 表示是否存在
func (p *Client) GetObject(ctx context.Context, key *Key, out any) (bool, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return false, err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		cmd = p.client.GetEx(ctx, k, key.TTL())
	} else {
		cmd = p.client.Get(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return false, nil
	}
	if cmd.Err() != nil {
		return false, cmd.Err()
	}
	bytes, err := cmd.Bytes()
	if err != nil {
		return false, err
	}
	if err := DefaultCodec().Unmarshal(bytes, out); err != nil {
		return false, err
	}
	return true, nil
}

// Exists 判断 key 是否存在
func (p *Client) Exists(ctx context.Context, key *Key) (bool, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return false, err
	}
	cnt, err := p.client.Exists(ctx, k).Result()
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// SetTTL 设置 key 的 TTL；若 key.TTL()<=0 则移除过期（PERSIST）
func (p *Client) SetTTL(ctx context.Context, key *Key) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	t := key.TTL()
	if t > 0 {
		_, err = p.client.Expire(ctx, k, t).Result()
		return err
	}
	_, err = p.client.Persist(ctx, k).Result()
	return err
}

// GetObjectsT 返回 []*T（使用包级 DefaultCodec），缺失项返回 nil；可选回源 getByKey，并在 writeBack 时将回源值写回缓存
func GetObjectsT[T any](ctx context.Context, client *Client, cfg *CacheConfig, keys []string, getByKey GetObjectByKeyFunc[T], writeBack bool) ([]*T, error) {
	if client == nil || client.client == nil {
		return nil, errors.New("client is nil")
	}
	if cfg == nil {
		return nil, errors.New("cache config is nil")
	}
	if len(keys) == 0 {
		return []*T{}, nil
	}

	pipe := client.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	useGetEx := cfg.RenewTTL() && cfg.TTL() > 0
	for i, rk := range keys {
		full := cfg.BuildKey(rk)
		if useGetEx {
			cmds[i] = pipe.GetEx(ctx, full, cfg.TTL())
		} else {
			cmds[i] = pipe.Get(ctx, full)
		}
	}

	_, err := pipe.Exec(ctx)
	_ = pipe.Close()

	if err != nil && err != redis.Nil {
		return nil, err
	}

	out := make([]*T, 0, len(keys))
	// 缺失项准备批量回写
	type wbItem struct {
		key  string
		data []byte
	}
	var writes []wbItem
	for i, cmd := range cmds {
		if cmd.Err() == nil {
			b, berr := cmd.Bytes()
			if berr != nil {
				return nil, berr
			}
			var obj T
			if err := DefaultCodec().Unmarshal(b, &obj); err != nil {
				return nil, err
			}
			out = append(out, &obj)
			continue
		}
		if cmd.Err() == redis.Nil {
			if getByKey != nil {
				val, e := getByKey(keys[i], i)
				if e != nil {
					return nil, e
				}
				out = append(out, val)
				if writeBack && val != nil {
					bytes, merr := DefaultCodec().Marshal(val)
					if merr != nil {
						logger.Errorf("marshal for write-back failed: %v", merr)
					} else {
						writes = append(writes, wbItem{key: cfg.BuildKey(keys[i]), data: bytes})
					}
				}
				continue
			}
			out = append(out, nil)
			continue
		}
		return nil, cmd.Err()
	}

	// 批量回写
	if writeBack && len(writes) > 0 {
		wb := client.client.Pipeline()
		for _, w := range writes {
			wb.Set(ctx, w.key, w.data, cfg.TTL())
		}
		if _, e := wb.Exec(ctx); e != nil && e != redis.Nil {
			logger.Errorf("write-back pipeline failed: %v", e)
		}
		_ = wb.Close()
	}
	return out, nil
}

// --- Hash (map) helpers ---

// HSetString sets a hash field to a string value; if key.TTL()>0, also sets/refreshes expiry to TTL.
func (p *Client) HSetString(ctx context.Context, key *Key, field string, value string) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	if key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.HSet(ctx, k, field, value)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		return err
	}
	return p.client.HSet(ctx, k, field, value).Err()
}

// HSetStrings sets multiple hash fields from a map[string]string; if key.TTL()>0, also sets/refreshes expiry to TTL.
func (p *Client) HSetStrings(ctx context.Context, key *Key, values map[string]string) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	// Convert to interface{} pairs
	args := make([]interface{}, 0, len(values)*2)
	for f, v := range values {
		args = append(args, f, v)
	}
	if key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.HSet(ctx, k, args...)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		return err
	}
	return p.client.HSet(ctx, k, args...).Err()
}

// HGetString gets a hash field as string; if key.RenewTTL() && TTL>0, also refreshes expiry.
// Returns "" and nil when field or key does not exist.
func (p *Client) HGetString(ctx context.Context, key *Key, field string) (string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return "", err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.HGet(ctx, k, field)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return "", err
		}
	} else {
		cmd = p.client.HGet(ctx, k, field)
	}
	if cmd.Err() == redis.Nil {
		return "", nil
	}
	return cmd.Result()
}

// HDel deletes one or more hash fields. Returns the number of fields removed.
func (p *Client) HDel(ctx context.Context, key *Key, fields ...string) (int64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	if len(fields) == 0 {
		return 0, nil
	}
	return p.client.HDel(ctx, k, fields...).Result()
}

// HExists determines if a hash field exists.
func (p *Client) HExists(ctx context.Context, key *Key, field string) (bool, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return false, err
	}
	return p.client.HExists(ctx, k, field).Result()
}

// HGetAll returns all fields and values of the hash as map[string]string; if key.RenewTTL() && TTL>0, also refreshes expiry.
func (p *Client) HGetAll(ctx context.Context, key *Key) (map[string]string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return nil, err
	}
	var cmd *redis.StringStringMapCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.HGetAll(ctx, k)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return nil, err
		}
	} else {
		cmd = p.client.HGetAll(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return map[string]string{}, nil
	}
	return cmd.Result()
}

// --- Set helpers ---

// SAdd adds members to a set; if key.TTL()>0, also sets/refreshes expiry to TTL.
func (p *Client) SAdd(ctx context.Context, key *Key, members ...string) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	// Convert []string to []interface{}
	args := make([]interface{}, len(members))
	for i := range members {
		args[i] = members[i]
	}
	if key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.SAdd(ctx, k, args...)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		return err
	}
	return p.client.SAdd(ctx, k, args...).Err()
}

// SRem removes members from a set. Returns the number of members removed.
func (p *Client) SRem(ctx context.Context, key *Key, members ...string) (int64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	if len(members) == 0 {
		return 0, nil
	}
	args := make([]interface{}, len(members))
	for i := range members {
		args[i] = members[i]
	}
	return p.client.SRem(ctx, k, args...).Result()
}

// SIsMember checks if member is in the set; if key.RenewTTL() && TTL>0, also refreshes expiry.
func (p *Client) SIsMember(ctx context.Context, key *Key, member string) (bool, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return false, err
	}
	var cmd *redis.BoolCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.SIsMember(ctx, k, member)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return false, err
		}
	} else {
		cmd = p.client.SIsMember(ctx, k, member)
	}
	if cmd.Err() == redis.Nil {
		return false, nil
	}
	return cmd.Result()
}

// SMembers returns all members of the set; if key.RenewTTL() && TTL>0, also refreshes expiry.
func (p *Client) SMembers(ctx context.Context, key *Key) ([]string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return nil, err
	}
	var cmd *redis.StringSliceCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.SMembers(ctx, k)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return nil, err
		}
	} else {
		cmd = p.client.SMembers(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return []string{}, nil
	}
	return cmd.Result()
}

// SCard returns the set cardinality.
func (p *Client) SCard(ctx context.Context, key *Key) (int64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	return p.client.SCard(ctx, k).Result()
}

// --- List helpers ---

// LPush pushes values to the head of the list; if key.TTL()>0, also sets/refreshes expiry to TTL.
func (p *Client) LPush(ctx context.Context, key *Key, values ...string) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	args := make([]interface{}, len(values))
	for i := range values {
		args[i] = values[i]
	}
	if key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.LPush(ctx, k, args...)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		return err
	}
	return p.client.LPush(ctx, k, args...).Err()
}

// RPush pushes values to the tail of the list; if key.TTL()>0, also sets/refreshes expiry to TTL.
func (p *Client) RPush(ctx context.Context, key *Key, values ...string) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	args := make([]interface{}, len(values))
	for i := range values {
		args[i] = values[i]
	}
	if key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.RPush(ctx, k, args...)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		return err
	}
	return p.client.RPush(ctx, k, args...).Err()
}

// LPop pops one element from the head; if key.RenewTTL() && TTL>0, also refreshes expiry.
func (p *Client) LPop(ctx context.Context, key *Key) (string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return "", err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.LPop(ctx, k)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return "", err
		}
	} else {
		cmd = p.client.LPop(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return "", nil
	}
	return cmd.Result()
}

// RPop pops one element from the tail; if key.RenewTTL() && TTL>0, also refreshes expiry.
func (p *Client) RPop(ctx context.Context, key *Key) (string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return "", err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.RPop(ctx, k)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return "", err
		}
	} else {
		cmd = p.client.RPop(ctx, k)
	}
	if cmd.Err() == redis.Nil {
		return "", nil
	}
	return cmd.Result()
}

// LLen returns the length of the list.
func (p *Client) LLen(ctx context.Context, key *Key) (int64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	return p.client.LLen(ctx, k).Result()
}

// LRange returns the specified elements of the list; if key.RenewTTL() && TTL>0, also refreshes expiry.
func (p *Client) LRange(ctx context.Context, key *Key, start, stop int64) ([]string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return nil, err
	}
	var cmd *redis.StringSliceCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.LRange(ctx, k, start, stop)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return nil, err
		}
	} else {
		cmd = p.client.LRange(ctx, k, start, stop)
	}
	if cmd.Err() == redis.Nil {
		return []string{}, nil
	}
	return cmd.Result()
}

// LIndex returns the element at index; if key.RenewTTL() && TTL>0, also refreshes expiry.
func (p *Client) LIndex(ctx context.Context, key *Key, index int64) (string, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return "", err
	}
	var cmd *redis.StringCmd
	if key.RenewTTL() && key.TTL() > 0 {
		_, err = p.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			cmd = pipe.LIndex(ctx, k, index)
			pipe.Expire(ctx, k, key.TTL())
			return nil
		})
		if err != nil && err != redis.Nil {
			return "", err
		}
	} else {
		cmd = p.client.LIndex(ctx, k, index)
	}
	if cmd.Err() == redis.Nil {
		return "", nil
	}
	return cmd.Result()
}

// LRem removes elements equal to value. Returns the number of removed elements.
func (p *Client) LRem(ctx context.Context, key *Key, count int64, value string) (int64, error) {
	k, err := p.validateParamKey(key)
	if err != nil {
		return 0, err
	}
	return p.client.LRem(ctx, k, count, value).Result()
}

// LTrim trims a list to the specified range.
func (p *Client) LTrim(ctx context.Context, key *Key, start, stop int64) error {
	k, err := p.validateParamKey(key)
	if err != nil {
		return err
	}
	return p.client.LTrim(ctx, k, start, stop).Err()
}
