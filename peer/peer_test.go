package peer

import "testing"

func TestPeer(t *testing.T) {
	p := NewPeer("127.0.0.1:8888")
	p.RegisterPeers("127.0.0.1:1111", "127.0.0.1:2222")

	strs := [5]string{"ayang", "tom", "HQUer", "cache", "ayangcache"}

	for i := range strs {
		if addr := p.GetPeer(strs[i]); addr == "" {
			println("本节点")
		} else {
			println(addr)
		}
	}
}
