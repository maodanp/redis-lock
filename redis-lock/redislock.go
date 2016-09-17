// Copyright 2016 maodanp, maodanp@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redislock

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/maodanp/go-log"
	_ "test/g"
	"time"
)

const (
	//DefaultName is used when Config Name is empty
	DefaultName = "redislock"

	// DefaultExpiry is used when Config Duration is 0
	// 锁的默认过期时间
	DefaultExpiry = 8 * time.Second

	//DefaultTimeOut is used when Config timeOut is 0
	// 默认调用Lock()函数的超时时间
	DefaultTimeOut = 24 * time.Second

	// DefaultDelay is used when Config Delay is 0
	// 默认获取锁的重试间隔
	DefaultDelay = 512 * time.Millisecond
)

type Config struct {
	// Resource Name
	Name string

	// Duration for which the lock is valid, DefaultExpiry if 0
	Expiry time.Duration

	// TimeOut for which the Lock() is called, DefaultTimeOut if 0
	TimeOut time.Duration

	// Delay between two attempts to acquire lock, DefaultDelay if 0
	Delay time.Duration
}

// RedisLock implements the distributed lock with one redis instance
type RedisLock struct {
	Config

	//Conn redis.Conn
	RedisPool *redis.Pool

	// ensure the lock will be removed only if it is still the one
	// that was set by the client trying to remove it
	Value int64
}

// NewRedisLock create a RedisLock struct
func NewRedisLock(RedisHostAndPort string, config Config) (*RedisLock, error) {
	redisPool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", fmt.Sprintf("%s", RedisHostAndPort))
			if err != nil {
				log.Logger.Warn("pkg.Dial", err)
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	if redisPool == nil {
		return nil, fmt.Errorf("redisPool is nil")
	}

	if config.Expiry == 0 {
		config.Expiry = DefaultExpiry
	}
	if config.Delay == 0 {
		config.Delay = DefaultDelay
	}
	if config.TimeOut == 0 {
		config.TimeOut = DefaultTimeOut
	}
	if config.Name == "" {
		config.Name = DefaultName
	}
	return &RedisLock{RedisPool: redisPool, Config: config}, nil
}

// Conn get a connection with redis from pool
func (r *RedisLock) Conn() redis.Conn {
	return r.RedisPool.Get()
}

// Close close a connection
func (r *RedisLock) Close() error {
	return r.RedisPool.Close()
}

// Lock attempts to get the distributed lock with lockName,
// Lock returns true when the goroutine get the lock or false when timeout elapses
func (r *RedisLock) Lock() (lock bool, err error) {
	until := time.Now().Add(r.TimeOut)
	for {
		// timeOut
		if time.Now().Before(until) == false {
			return false, fmt.Errorf("timeout")
		}
		curTime := time.Now().UnixNano() / int64(time.Millisecond)
		succ, _ := r.cmdSetnx(r.Name, curTime)
		if succ {
			r.Value = curTime
			return true, nil
		}

		var valGet, valGetSet int64
		valGet, err = r.cmdGet(r.Name)
		// the lock is deleted when cmd GET returs err(nil returned)
		// so sleep and retry to run cmd SETNX
		// 锁已经被释放，则重新竞争
		if err != nil {
			goto LOCK_RETRY
		} else {
			// the lock is captured now
			// 锁还被占用着，如果锁未达到超时时间，则重新竞争
			if int64(r.Expiry/time.Millisecond) > curTime-valGet {
				goto LOCK_RETRY
			}
		}

		// the lock is timeout, so involve the race
		// 存储新的，返回旧的值
		valGetSet, err = r.cmdGetSet(r.Name, curTime)
		// the lock is deleted when cmd GETSET returs err(nil returned)
		// so sleep and retry to run cmd SETNX
		// 可能到这一步锁正好已经释放了，则重新竞争
		if err != nil {
			goto LOCK_RETRY
		}

		// haha, I get the lock!!
		// 如果GET的值与GETSET的值相等，则该协程获得了锁
		if valGet == valGetSet {
			r.Value = valGet
			return true, nil
		}

	LOCK_RETRY:
		time.Sleep(r.Delay)
	}
	return
}

func (r *RedisLock) UnLock() (succ bool, err error) {
	return r.cmdDel(r.Name)
}

func (r *RedisLock) cmdSetnx(key string, val int64) (succ bool, _ error) {
	conn := r.RedisPool.Get()
	defer conn.Close()

	reply, err := redis.Int64(conn.Do("SETNX", key, val))
	if err != nil {
		return false, err
	}
	log.Logger.Debug("RedisLock.cmdSetnx reply ", reply)
	if reply == int64(0) {
		return false, nil
	}
	return true, nil
}

func (r *RedisLock) cmdGet(key string) (retVal int64, err error) {
	conn := r.RedisPool.Get()
	defer conn.Close()

	reply, err := redis.Int64(conn.Do("GET", key))
	if err != nil {
		return 0, err
	}
	log.Logger.Debug("RedisLock.cmdGet replay ", reply)

	return reply, nil
}

func (r *RedisLock) cmdGetSet(key string, val int64) (retVal int64, err error) {
	conn := r.RedisPool.Get()
	defer conn.Close()

	retVal, err = redis.Int64(conn.Do("GETSET", key, val))
	if err != nil {
		return 0, err
	}
	log.Logger.Debug("RedisLock.cmdGetSet replay ", retVal)

	return
}

// cmdDel
func (r *RedisLock) cmdDel(key string) (succ bool, err error) {
	conn := r.RedisPool.Get()
	defer conn.Close()

	// avoid removing a lock that was created by another client.
	reply, err := redis.Int64(delScript.Do(conn, key, r.Value))
	if err != nil {
		log.Logger.Warn("RedisLock.cmdDel.Do ", err)
		return false, err
	}
	if reply == 0 {
		return false, err
	}
	log.Logger.Debug("RedisLock.cmdDel reply ", reply)
	return true, nil
}

var delScript = redis.NewScript(1, `
if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("del", KEYS[1])
        else
  return 0
end`)
