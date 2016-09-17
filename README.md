# redis-lock
redis-lock provides a distributed mutual exclusion lock implementation for Go, the lock is based on single redis instance, not based on the redis cluster.

It's very simple to use, but if you consider the mutual exclusion lock used in redis cluster, you need use [Redsync.go](https://github.com/hjr265/redsync.go).

## Installation

install redis-lock using the go get command:

	$ go get github.com/maodanp/redis-lock

The dependencies are the github.com/maodanp/go-log and github.com/garyburd/redigo/redis.

## Example Usage
~~~go
func simpleTest() {
client, err := NewRedisLock("127.0.0.1:6379", Config{})
defer client.Close()
	if err != nil {
			return
	}

if lock, err := client.Lock(); err != nil {
	log.Logger.Warnf("%d lock err %v", idx, err)
		return
} else if lock {
	log.Logger.Infof("%d locked", idx)
		// time.Sleep mock the work which goroutine do
		time.Sleep(time.Second)
		client.UnLock()

}
}
~~~

## License

kingshard is under the Apache 2.0 license.
