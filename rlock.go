package rlock

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"sync"
	"time"
)

const LOCK_DEL = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`

var (
	DefaultInstance *Rlock
	mux             = &sync.Mutex{}
	once            sync.Once
	lockDelHash     = ""
	defaultConfig   = Config{
		Prefix:      "RLOCK_",
		DisableHash: false,
	}
)

type Rlock struct {
	Config
	cli  *redis.Client
	hash string
}

type RlockContext struct {
	Ctx context.Context
	Key string
	Val string
}

type Config struct {
	Prefix      string
	DisableHash bool
}

type CancelFunc func() bool

func SetDefault(rcli *redis.Client, cfgs ...Config) {
	mux.Lock()
	defer mux.Unlock()
	DefaultInstance = New(rcli, cfgs...)
}

func New(rcli *redis.Client, cfgs ...Config) *Rlock {
	cfg := defaultConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	if !cfg.DisableHash {
		once.Do(func() {
			hashstr, err := rcli.ScriptLoad(context.TODO(), LOCK_DEL).Result()
			if err != nil {
				lockDelHash = hashstr
			}
		})
	}
	return &Rlock{cli: rcli, Config: cfg, hash: lockDelHash}
}

func Acquire(lockName string, timeout time.Duration) (bool, CancelFunc) {
	return DefaultInstance.Acquire(lockName, timeout)
}

func (rlock *Rlock) Acquire(lockName string, timeout time.Duration) (bool, CancelFunc) {
	key := fmt.Sprintf("%s%s", rlock.Prefix, lockName)
	val := fmt.Sprintf("%s_%s", lockName, uuid.NewString())
	endtime := time.Now().UnixMicro() + timeout.Microseconds()

	ctx := context.TODO()

	var cancelFunc = func() bool {
		r, err := rlock.cancel(ctx, key, val)
		if err != nil {
			return false
		}
		if reply, ok := r.(int64); !ok {
			return false
		} else {
			return reply == 1
		}
	}

	tout := timeout + time.Millisecond*500
	for {
		if time.Now().UnixMicro() > endtime {
			return false, cancelFunc
		}
		ok, er := rlock.cli.SetNX(ctx, key, val, tout).Result()
		if er != nil {
			return false, cancelFunc
		}
		if ok {
			return true, cancelFunc
		}
		time.Sleep(time.Millisecond * 5)
	}
}

func (rlock *Rlock) cancel(ctx context.Context, key, val string) (r interface{}, err error) {
	if rlock.hash == "" {
		r, err = rlock.cli.Eval(ctx, LOCK_DEL, []string{key}, []any{val}).Result()
	} else {
		r, err = rlock.cli.EvalSha(ctx, rlock.hash, []string{key}, []any{val}).Result()
	}
	return
}
