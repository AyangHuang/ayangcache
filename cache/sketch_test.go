package cache

import (
	"fmt"
	"testing"
)

func TestCmSketch(t *testing.T) {
	cm := newCmSketch(4 * 10)

	hashKey1, _ := KeyToHash("ayang")
	hashKey2, _ := KeyToHash("tom")
	hashKey3, _ := KeyToHash("linux")
	hashKey4, _ := KeyToHash("Go")

	// 3 次
	cm.Increment(hashKey1)
	cm.Increment(hashKey1)
	cm.Increment(hashKey1)

	// 6 次
	cm.Increment(hashKey2)
	cm.Increment(hashKey2)
	cm.Increment(hashKey2)
	cm.Increment(hashKey2)
	cm.Increment(hashKey2)
	cm.Increment(hashKey2)

	// 1 次
	cm.Increment(hashKey3)

	// 15 次
	for i := 0; i < 15; i++ {
		cm.Increment(hashKey4)
	}

	fmt.Printf("k1:%d, k2:%d, k3:%d, k4:%d\n", cm.Estimate(hashKey1), cm.Estimate(hashKey2), cm.Estimate(hashKey3), cm.Estimate(hashKey4))

	cm.Reset()

	fmt.Printf("k1:%d, k2:%d, k3:%d, k4:%d\n", cm.Estimate(hashKey1), cm.Estimate(hashKey2), cm.Estimate(hashKey3), cm.Estimate(hashKey4))

	cm.Clear()

	fmt.Printf("k1:%d, k2:%d, k3:%d, k4:%d\n", cm.Estimate(hashKey1), cm.Estimate(hashKey2), cm.Estimate(hashKey3), cm.Estimate(hashKey4))
}
