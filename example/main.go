package main

import (
	"context"
	"fmt"
	"github.com/eininst/rlock"
	"time"
)

func main() {
	rlock.SetDefault(rlock.New("redis://localhost:6379/0"))

	//Leverage Go's `context.Context` for cancellation and timeout control
	//The default timeout is the key expiration time.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ok, unlock := rlock.Acquire(ctx, "lock_name_test")
	defer unlock()

	if ok {
		fmt.Println("my is safe")
	}
}
