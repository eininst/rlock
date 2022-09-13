package rlock

import (
	"context"
	"fmt"
	"github.com/eininst/flog"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"sync"
	"time"
)

var rlog flog.Interface

func init() {
	f := fmt.Sprintf("${time} ${level} %s[RLOCK]%s ${msg}", flog.Magenta, flog.Reset)
	rlog = flog.New(flog.Config{
		Format: f,
	})
}

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
	defaultCancelFunc = func() bool {
		return false
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
	hash := ""
	if !cfg.DisableHash {
		once.Do(func() {
			hashstr, err := rcli.ScriptLoad(context.TODO(), LOCK_DEL).Result()
			if err != nil {
				rlog.Error("[RLOCK] Script load error:", err)
			} else {
				lockDelHash = hashstr
			}
		})
		hash = lockDelHash
	}
	return &Rlock{cli: rcli, Config: cfg, hash: hash}
}

func Acquire(lockName string, timeout time.Duration) (bool, CancelFunc) {
	return DefaultInstance.Acquire(lockName, timeout)
}

func (rlock *Rlock) TryAcquire(lockName string, timeout time.Duration) (bool, CancelFunc) {
	ctx := context.TODO()
	key := fmt.Sprintf("%s%s", rlock.Prefix, lockName)
	val := fmt.Sprintf("%s_%s", lockName, uuid.NewString())
	ok, er := rlock.cli.SetNX(ctx, key, val, timeout).Result()
	if er != nil {
		rlog.Errorf(`SetNX key: "%s", Error: %v`, key, er)
		return false, defaultCancelFunc
	}
	if ok {
		return true, func() bool {
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
	}
	return false, defaultCancelFunc
}
func (rlock *Rlock) Acquire(lockName string, timeout time.Duration) (bool, CancelFunc) {
	ctx := context.TODO()
	key := fmt.Sprintf("%s%s", rlock.Prefix, lockName)
	val := fmt.Sprintf("%s_%s", lockName, uuid.NewString())
	endtime := time.Now().UnixMicro() + timeout.Microseconds()

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

	for {
		if time.Now().UnixMicro() > endtime {
			return false, defaultCancelFunc
		}
		ok, er := rlock.cli.SetNX(ctx, key, val, timeout).Result()
		if er != nil {
			rlog.Errorf(`SetNX key: "%s", Error: %v`, key, er)
			return false, defaultCancelFunc
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
