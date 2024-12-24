# Rlock

[![Go Reference](https://pkg.go.dev/badge/github.com/eininst/rlock.svg)](https://pkg.go.dev/github.com/eininst/rlock)
[![License](https://img.shields.io/github/license/eininst/rlock.svg)](LICENSE)

`rlock` is a Go package that provides a simple and efficient distributed locking mechanism using Redis. It allows multiple instances of your application to coordinate access to shared resources, ensuring that only one instance can hold the lock at any given time.

## Features

- **Distributed Locking**: Utilize Redis to manage distributed locks across multiple instances.
- **Customizable Options**: Configure retry attempts, retry delays, and lock expiration times.
- **Safe Lock Release**: Employs Lua scripting to ensure that only the lock owner can release the lock.
- **Flexible Initialization**: Initialize using a Redis URL or an existing Redis client.
- **Context Support**: Leverage Go's `context.Context` for cancellation and timeout control.


## ⚙️ Installation

```text
go get -u github.com/eininst/rlock
```

## ⚡ Quickstart

```go
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
```

## 👀 New a Instance

```go
func main() {
    lk := rlock.New("redis://localhost:6379/0", 
	rlock.WithExpiration(time.Second*10),
        rlock.WithMaxRetry(200),
        rlock.WithRetryDelay(time.Millisecond*100),)
    
    ok, cancel := lk.Acquire("lock_name_test")
    defer cancel()
    if ok {
        fmt.Println("my is safe")
    }
}
```

> See [examples](/example)

## License

*MIT*