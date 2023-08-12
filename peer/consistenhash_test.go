package peer

import (
	"fmt"
	"testing"
)

func TestMap(t *testing.T) {
	g := NewMap(1024, nil)
	g.Init("127.0.0.1:9999", "127.0.0.1:8888")
	fmt.Println(g.Get("ayang"))
	fmt.Println(g.Get("tom"))
}

func TestMapHashConflict(t *testing.T) {
	// 本来想 replaces = 2 << 31，无奈超出内存了
	// 20 的结果 * 2 == 21 的结果，对的话基本可以判断没有错误
	g := NewMap(2<<20, nil)
	g.Init("test1", "test2")
	fmt.Println(len(g.virtualRing))
}
