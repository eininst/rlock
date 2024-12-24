package rlock

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/url"
	"strconv"
	"time"
)

const DefaultRedisPoolSize = 1024

type Option struct {
	F func(o *Options)
}

type Options struct {
	MaxRetry   int
	RetryDelay time.Duration
	Expiration time.Duration
}

func (o *Options) Apply(opts []Option) {
	for _, op := range opts {
		op.F(o)
	}
}

func WithMaxRetry(maxRetry int) Option {
	return Option{F: func(o *Options) {
		o.MaxRetry = maxRetry
	}}
}

func WithRetryDelay(retryDelay time.Duration) Option {
	return Option{F: func(o *Options) {
		o.RetryDelay = retryDelay
	}}
}

func WithExpiration(expiration time.Duration) Option {
	return Option{F: func(o *Options) {
		o.Expiration = expiration
	}}
}

type RedisOptions struct {
	Scheme   string
	Addr     string
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// ParseRedisURL parses the Redis URI and returns RedisOptions.
// It ensures robust error handling and uses default values where necessary.
func ParseRedisURL(uri string) (*RedisOptions, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	scheme := parsedURL.Scheme
	host := parsedURL.Hostname()
	portStr := parsedURL.Port()
	password, _ := parsedURL.User.Password()

	// Default Redis port
	port := 6379
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port in Redis URL: %w", err)
		}
	}

	// Parse DB index from the path, defaulting to 0
	db := 0
	if len(parsedURL.Path) > 1 {
		dbStr := parsedURL.Path[1:]
		db, err = strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DB index in Redis URL: %w", err)
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	return &RedisOptions{
		Scheme:   scheme,
		Addr:     addr,
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
		PoolSize: DefaultRedisPoolSize,
	}, nil
}

// NewRedisClient creates a new Redis client based on the provided URI.
// It returns an error if the URI is invalid or if the connection fails.
func newRedisClient(uri string) (*redis.Client, error) {
	opt, err := ParseRedisURL(uri)
	if err != nil {
		return nil, fmt.Errorf("error parsing Redis URL: %w", err)
	}

	rcli := redis.NewClient(&redis.Options{
		Addr:     opt.Addr,
		Password: opt.Password,
		DB:       opt.DB,
		PoolSize: opt.PoolSize,
	})

	// Verify the connection by pinging the Redis server
	if err = rcli.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return rcli, nil
}
