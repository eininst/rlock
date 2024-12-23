# Rlock

[![Go Reference](https://pkg.go.dev/badge/github.com/eininst/rlock.svg)](https://pkg.go.dev/github.com/eininst/redis-stream-pubsub)
[![License](https://img.shields.io/github/license/eininst/rlock.svg)](LICENSE)

`Implementation of redis lock`

## ⚙️ Installation

```text
go get -u github.com/eininst/rlock
```

## ⚡ Quickstart

```go
func init(){
    rlock.SetDefault(getRedis())
}

func main() {
    ok, cancel := rlock.Acquire("lock_name_test", time.Second*10)
    defer cancel()
    if ok {
        fmt.Println("my is safe")
    }
}
```

## 👀 New a Instance

```go
func main() {
    lk := rlock.New(getRedis())
    
    ok, cancel := lk.Acquire("lock_name_test", time.Second*10)
    defer cancel()
    if ok {
        fmt.Println("my is safe")
    }
}
```

> See [examples](/examples)

## License

*MIT*