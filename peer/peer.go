package peer

import "sync"

const (
	virtualPeerNum = 256
)

type Peer interface {
	// GetPeer 获取分布式节点，"" 表示本节点
	GetPeer(key string) string
}

type peer struct {
	rw sync.RWMutex
	// 本节点地址
	addr string
	m    *Map
	// 注册中心
	register RegistrationCenterClient
}

func NewPeer(localAddr, registerAddr string) Peer {
	p := &peer{
		addr:     localAddr,
		m:        NewMap(virtualPeerNum, nil),
		register: NewEtcdRegistrationCenterClient(localAddr, registerAddr),
	}

	// 阻塞等待服务注册和第一次
	notifyChan := p.register.Notify()
	p.initPeers(<-notifyChan...)

	// 后面监听
	go func() {
		for {
			select {
			case nodes := <-notifyChan:
				p.initPeers(nodes...)
			}
		}
	}()
	return p
}

func (p *peer) initPeers(addr ...string) {
	p.rw.Lock()
	p.m.Init(addr...)
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
