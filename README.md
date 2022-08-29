# RLOCK

[![Build Status](https://travis-ci.org/ivpusic/grpool.svg?branch=master)](https://github.com/infinitasx/easi-go-aws)

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
    ok, cancel := lock.Acquire("lock_name_test", time.Second*10)
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