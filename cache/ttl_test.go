package cache

import (
	"fmt"
	"testing"
	"time"
)

type mockStore struct {
	store
}

type mockPolicy struct {
	policy
}

// Del 覆盖方法，编译时会自动选择最近的，也就是这个方法
func (*mockStore) Del(hashKey uint64, conflict uint64) (interface{}, bool) {
	fmt.Printf("Del hashKey:%d conflict:%d\n", hashKey, conflict)
	return nil, false
}

func (mockPolicy) Del(uint64) {}

func TestClean(t *testing.T) {
	s := newExpirationMap()

	hashKey, conflict := KeyToHash("ayang")
	hashKey1, conflict1 := KeyToHash("tom")
	s.Add(hashKey, conflict, time.Now().Add(time.Millisecond))
	s.Add(hashKey1, conflict1, time.Now().Add(time.Duration(secondsPerBucket)*time.Second))

	for i := 0; i < 4; i++ {
		time.Sleep(time.Duration(secondsPerBucket) * time.Second)
		// 代码量少，且没有其他协程抢占，时间都是准的，所有极大概率会在第 2 和第 3 次各删除一个
		println(i)
		s.Clean(&mockStore{}, mockPolicy{})
	}
}
