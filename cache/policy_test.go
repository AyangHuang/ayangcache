package cache

import (
	"fmt"
	"testing"
)

type mockDefaultPolicy struct {
	*defaultPolicy
}

// 重写 ConsumeGet 方法
func (mock *mockDefaultPolicy) ConsumeGet(hashKeys []uint64) bool {
	for i := range hashKeys {
		mock.admit.incrementFre(hashKeys[i])
	}
	return true
}

func (mock *mockDefaultPolicy) getAllKeys() []uint64 {
	all := make([]uint64, 0, len(mock.evict.keyCosts))
	for hashKey, _ := range mock.evict.keyCosts {
		all = append(all, hashKey)
	}
	return all
}

func TestDefaultPolicy(t *testing.T) {
	policy := &mockDefaultPolicy{
		defaultPolicy: newDefaultPolicy(4*10, 4),
	}

	policy.Add(1, 1)
	policy.Add(2, 1)
	policy.Add(3, 1)
	policy.Add(4, 1)

	fmt.Println(policy.getAllKeys()) // 1, 2, 3, 4

	policy.ConsumeGet([]uint64{3, 3, 4, 4, 5})
	if item, ok := policy.Add(5, 2); ok {
		fmt.Println(item)                // 大概率 1, 2
		fmt.Println(policy.getAllKeys()) // 大概率 3，4，5
		fmt.Println(policy.evict.used)   // 大概率 4
	}
}
