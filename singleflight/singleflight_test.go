package singleflight

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroup_Do(t *testing.T) {
	redis, localCache := map[string]string{}, map[string]string{}
	result := "18"
	redis["ayang"] = result

	w := &sync.WaitGroup{}
	w.Add(1000)

	// Do 获取成功的次数
	count := atomic.Int32{}
	// 从 redis 获取的次数
	getCount := atomic.Int32{}

	g := NewGroup()

	for i := 0; i < 1000; i++ {
		go func() {
			if str, err := g.Do("ayang", func() (interface{}, error) {
				// 再检查一次，因为外面的逻辑：（检查缓存没有，执行 Do）不是原子的
				if str, ok := localCache["ayang"]; ok {
					return str, nil
				}

				// 表示从远程获取
				time.Sleep(time.Second)
				getCount.Add(1)
				if str, ok := redis["ayang"]; ok {
					return str, nil
				} else {
					return nil, nil
				}

			}); err == nil {
				if str == result {
					count.Add(1)
				}
			}
			w.Done()
		}()
	}

	w.Wait()

	if count.Load() != 1000 {
		t.Error("获取成功次数没达到 1000")
	}
	if getCount.Load() != 1 {
		t.Error("执行获取次数超过 1 次")
	}
}
