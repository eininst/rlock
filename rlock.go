package rlock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/eininst/flog"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// 1) 全局初始化日志
var rlog flog.Interface

func init() {
	format := fmt.Sprintf("${time} ${level} %s[RLOCK]%s ${msg}", flog.Magenta, flog.Reset)
	rlog = flog.New(flog.Config{Format: format})
}

// 2) 定义 Lua 脚本与默认常量
const lockDelScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

var (
	defaultInstance   *Rlock
	defaultInstanceMu sync.Mutex
	scriptLoadOnce    sync.Once

	lockDelScriptHash string

	lockInterval = 4 * time.Millisecond // 默认轮询间隔

	defaultConfig = Config{
		Prefix: "RLOCK_",
	}

	// 当加锁失败或不需要解锁时的占位函数
	defaultCancelFunc = func() bool { return false }
)

// 3) Rlock 相关结构
type Rlock struct {
	Config
	cli *redis.Client
}

type Config struct {
	Prefix string
}

// 用于记录上下文/键值
type RlockContext struct {
	Ctx context.Context
	Key string
	Val string
}

// 解锁函数类型
type CancelFunc func() bool

// 4) 全局方法 SetDefault，可设置单例锁实例
func SetDefault(rcli *redis.Client, cfgs ...Config) {
	defaultInstanceMu.Lock()
	defer defaultInstanceMu.Unlock()
	defaultInstance = New(rcli, cfgs...)
}

// 5) 创建新的 Rlock 实例，并在首次调用时加载 Lua 脚本
func New(rcli *redis.Client, cfgs ...Config) *Rlock {
	cfg := defaultConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	scriptLoadOnce.Do(func() {
		sha, err := rcli.ScriptLoad(context.Background(), lockDelScript).Result()
		if err != nil {
			flog.Warn("[RLOCK] Failed to load Lua script:", err)
		} else {
			lockDelScriptHash = sha
		}
	})

	return &Rlock{
		cli:    rcli,
		Config: cfg,
	}
}

// 6) 方便使用的全局函数
func Acquire(lockName string, timeout time.Duration) (bool, CancelFunc) {
	if defaultInstance == nil {
		rlog.Error("defaultInstance is not set. Use SetDefault(...) first.")
		return false, defaultCancelFunc
	}
	return defaultInstance.Acquire(lockName, timeout)
}

func TryAcquire(lockName string, expire time.Duration) (bool, CancelFunc) {
	if defaultInstance == nil {
		rlog.Error("defaultInstance is not set. Use SetDefault(...) first.")
		return false, defaultCancelFunc
	}
	return defaultInstance.TryAcquire(lockName, expire)
}

// 7) 不做重试的加锁, SetNX 失败直接返回
func (rl *Rlock) TryAcquire(lockName string, expire time.Duration) (bool, CancelFunc) {
	ctx := context.Background()
	key := rl.Config.Prefix + lockName
	val := fmt.Sprintf("%s_%s", lockName, uuid.NewString())

	ok, err := rl.cli.SetNX(ctx, key, val, expire).Result()
	if err != nil {
		rlog.Errorf(`TryAcquire SetNX key="%s" error: %v`, key, err)
		return false, defaultCancelFunc
	}

	if !ok {
		// 已存在，不可获取
		return false, defaultCancelFunc
	}

	// 获取成功，返回解锁函数
	return true, func() bool {
		return rl.release(ctx, key, val)
	}
}

// 8) 带重试的加锁，在 timeout 时间内反复尝试 SetNX
func (rl *Rlock) Acquire(lockName string, timeout time.Duration) (bool, CancelFunc) {
	ctx := context.Background()
	key := rl.Config.Prefix + lockName
	val := fmt.Sprintf("%s_%s", lockName, uuid.NewString())

	// 计算允许重试的截止时间
	deadline := time.Now().Add(timeout)

	// 锁的过期时间略长于重试时长
	expire := timeout + lockInterval

	for {
		if time.Now().After(deadline) {
			// 超过 deadline，放弃
			return false, defaultCancelFunc
		}
		ok, err := rl.cli.SetNX(ctx, key, val, expire).Result()
		if err != nil {
			rlog.Errorf(`Acquire SetNX key="%s" error: %v`, key, err)
			return false, defaultCancelFunc
		}

		if ok {
			// 成功获取锁
			return true, func() bool {
				return rl.release(ctx, key, val)
			}
		}
		// 未获取到锁, 等待后重试
		time.Sleep(lockInterval)
	}
}

// 9) 释放锁操作，优先使用脚本SHA，否则用 Eval
func (rl *Rlock) release(ctx context.Context, key, val string) bool {
	var (
		res interface{}
		err error
	)

	if lockDelScriptHash != "" {
		// 已载入脚本，使用 EvalSha
		res, err = rl.cli.EvalSha(ctx, lockDelScriptHash, []string{key}, val).Result()
	} else {
		// 未载入脚本或载入失败，使用原始脚本
		res, err = rl.cli.Eval(ctx, lockDelScript, []string{key}, val).Result()
	}

	if err != nil {
		rlog.Errorf("release lock failed, key=%s val=%s err=%v", key, val, err)
		return false
	}

	if reply, ok := res.(int64); ok {
		return reply == 1
	}
	rlog.Warnf("release lock unexpected return: %v", res)
	return false
}
