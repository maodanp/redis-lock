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
	"github.com/maodanp/go-log"
	"runtime"
	"sync"
	"testing"
	"time"
)

const CLIENT_CNT = 10

// in normal situation，the goroutine who get one lock
// can unlock when it finishes work
func TestLockNormal(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var wg sync.WaitGroup
	wg.Add(CLIENT_CNT)

	// how many goroutine get the lock in 10 second
	lockCnt := 0
	// flag will send to cntChan when the goroutine get the lock
	cntChan := make(chan int)
	for i := 0; i < CLIENT_CNT; i++ {
		go func(cntChan chan<- int, idx int) {
			defer wg.Done()

			client, err := NewRedisLock("127.0.0.1:6379", Config{})
			defer client.Close()
			if err != nil {
				log.Logger.Warn("new redis lock err: %vv", err)
				return
			}

			//if lock, err := client.Lock(LOCK_NAME, 10000, 11000, 1000); err != nil
			if lock, err := client.Lock(); err != nil {
				log.Logger.Warnf("%d lock err %v", idx, err)
				return
			} else if lock {
				log.Logger.Infof("%d locked", idx)
				// time.Sleep mock the work which goroutine do
				time.Sleep(time.Second)
				client.UnLock()
				cntChan <- 1

			}
		}(cntChan, i)
	}

	go func(cntChan <-chan int) {
		for {
			lockCnt += <-cntChan
		}
	}(cntChan)

	wg.Wait()
	time.Sleep(2 * time.Second)
	//log.Logger.Debugf("There are %d goroutines get the lock in 10 s", lockCnt)
	if lockCnt != CLIENT_CNT {
		t.Fatalf("the lockCnt [%d] is not equal with CLINET_CNT [%d]", lockCnt, CLIENT_CNT)
	}
}

// in exception situation，the goroutine who get one lock
// random invoke unlock after sleep
func TestLockException(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var wg sync.WaitGroup
	wg.Add(CLIENT_CNT)

	// how many goroutine get the lock in 10 second
	lockCnt := 0
	// flag will send to cntChan when the goroutine get the lock
	cntChan := make(chan int)
	for i := 0; i < CLIENT_CNT; i++ {
		go func(cntChan chan<- int, idx int) {
			defer wg.Done()

			client, err := NewRedisLock("127.0.0.1:6379", Config{Name: "testlockexception", Expiry: 3 * time.Second, TimeOut: 30 * time.Second, Delay: 100 * time.Millisecond})
			//client, err := NewRedisLock("127.0.0.1:6379", Config{})
			defer client.Close()
			if err != nil {
				log.Logger.Warn("new redis lock err: %vv", err)
				return
			}

			if lock, err := client.Lock(); err != nil {
				log.Logger.Warn("lock err ", err)
				return
			} else if lock {
				log.Logger.Infof("%d locked", idx)
				// time.Sleep mock the work which goroutine do
				// Here we remove unlock
				time.Sleep(time.Second)
				if idx&1 == 1 {
					log.Logger.Infof("%d delete lock", idx)
					client.UnLock()
				}
				cntChan <- 1

			}
		}(cntChan, i)
	}

	go func(cntChan <-chan int) {
		for {
			lockCnt += <-cntChan
		}
	}(cntChan)

	wg.Wait()
	time.Sleep(2 * time.Second)
	if lockCnt != CLIENT_CNT {
		t.Fatalf("the lockCnt [%d] is not equal with CLINET_CNT [%d]", lockCnt, CLIENT_CNT)
	}
}
