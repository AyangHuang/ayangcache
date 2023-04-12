package cache

import (
	"fmt"
	"testing"
)

type mockConsumer struct {
}

func (mockConsumer) ConsumeGet(hashKeys []uint64) bool {
	fmt.Println(hashKeys)
	return false
}

func TestRingBufferPool_Push_True(t *testing.T) {
	r := newRingBufferPool(mockConsumer{}, 2)
	for i := uint64(0); i < 5; i++ {
		r.Push(i)
	}
}
