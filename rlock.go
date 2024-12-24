package rlock

import (
	"context"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"log"
	"sync"
	"time"
)

var (
	// Lua 脚本用于释放锁
	unlockScript = redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)
	defaultCancelFunc = func() bool { return false }
	instance          *RedisLock
	onece             sync.Once
)

type RedisLock struct {
	options *Options
	rcli    *redis.Client
}

// 默认选项配置
func defaultOptions() *Options {
	return &Options{
		MaxRetry:   64,
		RetryDelay: 32 * time.Millisecond,
		Expiration: 8 * time.Second,
	}
}

func New(redisUrl string, opts ...Option) *RedisLock {
	_options := defaultOptions()
	_options.Apply(opts)

	rcli, er := newRedisClient(redisUrl)
	if er != nil {
		log.Fatal(er)
	}

	return &RedisLock{
		rcli:    rcli,
		options: _options,
	}
}

func NewWithClient(client *redis.Client, opts ...Option) *RedisLock {
	_options := defaultOptions()
	_options.Apply(opts)

	return &RedisLock{
		rcli:    client,
		options: _options,
	}
}

func SetDefault(rlock *RedisLock) {
	onece.Do(func() {
		instance = rlock
	})
}

func (r *RedisLock) Acquire(ctx context.Context, key string, opts ...Option) (bool, CancelFunc) {
	options := r.cloneOptions(opts...)
	rlk := newRedisLock(r.rcli, key, options)

	return rlk.Acquire(ctx)
}

func (r *RedisLock) TryAcquire(ctx context.Context, key string, opts ...Option) (bool, CancelFunc) {
	options := r.cloneOptions(opts...)
	rlk := newRedisLock(r.rcli, key, options)

	return rlk.TryAcquire(ctx)
}

func TryAcquire(ctx context.Context, key string, opts ...Option) (bool, CancelFunc) {
	return instance.TryAcquire(ctx, key, opts...)
}

func Acquire(ctx context.Context, key string, opts ...Option) (bool, CancelFunc) {
	return instance.Acquire(ctx, key, opts...)
}

func (r *RedisLock) cloneOptions(opts ...Option) *Options {
	newOptions := *r.options
	newOptions.Apply(opts)
	return &newOptions
}

// DistributedLock 分布式锁结构体
type redisLock struct {
	client     *redis.Client
	key        string
	value      string
	expiration time.Duration
	retryDelay time.Duration
	maxRetry   int
}

type CancelFunc func() bool

func newRedisLock(client *redis.Client, key string, opt *Options) *redisLock {
	return &redisLock{
		client:     client,
		key:        key,
		value:      uuid.New().String(), // 生成唯一标识符
		expiration: opt.Expiration,
		retryDelay: opt.RetryDelay,
		maxRetry:   opt.MaxRetry,
	}
}

func (l *redisLock) Acquire(ctx context.Context) (bool, CancelFunc) {
	for i := 0; i < l.maxRetry; i++ {
		ok, cancel := l.TryAcquire(ctx)

		if ok {
			return true, cancel
		}
		// Wait before retrying
		time.Sleep(l.retryDelay)
	}
	return false, defaultCancelFunc
}

// Lock 尝试获取锁
func (l *redisLock) TryAcquire(ctx context.Context) (bool, CancelFunc) {
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.expiration).Result()

	if err != nil {
		return false, defaultCancelFunc
	}
	if !ok {
		return false, defaultCancelFunc
	}

	return ok, func() bool {
		return l.unlock(ctx)
	}
}

// Unlock 释放锁
func (l *redisLock) unlock(ctx context.Context) bool {
	result, err := unlockScript.Run(ctx, l.client, []string{l.key}, l.value).Result()
	if err != nil {
		return false
	}
	if result.(int64) == 1 {
		return true
	}
	return false
}
