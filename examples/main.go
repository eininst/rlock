package main

import (
	"fmt"
	"github.com/eininst/rlock"
	"github.com/go-redis/redis/v8"
	"time"
)

func getRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		DB:           0,
		DialTimeout:  30 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     100,
		MinIdleConns: 25,
		PoolTimeout:  30 * time.Second,
	})
}

func init() {
	rlock.SetDefault(getRedis())
}

func main() {
	ok, cancel := rlock.Acquire("lock_name_test", time.Second*10)
	defer cancel()
	if ok {
		fmt.Println("my is safe")
	}
}
