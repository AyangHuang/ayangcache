package peer

import "sync"

const (
	virtualPeerNum = 256
)

type Peer interface {
	// GetPeer 获取分布式节点，"" 表示本节点
	GetPeer(key string) string
	RegisterPeers(addr ...string)
}

type peer struct {
	rw sync.RWMutex
	// 本节点地址
	addr string
	m    *Map
}

func NewPeer(addr string) Peer {
	p := &peer{
		addr: addr,
		m:    NewMap(virtualPeerNum, nil),
	}
	p.m.Add(addr)
	return p
}

func (p *peer) RegisterPeers(addr ...string) {
	p.rw.Lock()
	p.m.Add(addr...)
	p.rw.Unlock()
}

func (p *peer) GetPeer(key string) string {
	p.rw.RLock()
	addr := p.m.Get(key)
	p.rw.RUnlock()

	// 不为本 peer 节点
	if addr != "" && addr != p.addr {
		return addr
	}

	return ""
}
