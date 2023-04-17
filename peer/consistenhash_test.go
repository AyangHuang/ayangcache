package peer

import (
	"fmt"
	"testing"
)

func TestMap(t *testing.T) {
	g := NewMap(1024, nil)
	g.Add("127.0.0.1:9999", "127.0.0.1:8888")
	fmt.Println(g.Get("ayang"))
	fmt.Println(g.Get("tom"))
}
