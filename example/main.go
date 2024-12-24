package main

import (
	"context"
	"fmt"
	"github.com/eininst/rlock"
)

func main() {
	rlock.SetDefault(rlock.New("redis://localhost:6379/0"))

	ok, cancel := rlock.Acquire(context.TODO(), "lock_name_test")
	defer cancel()

	if ok {
		fmt.Println("my is safe")
	}
}
