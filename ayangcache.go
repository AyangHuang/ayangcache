package ayangcache

import (
	"fmt"
	"github.com/ayanghuang/ayangcache/byteview"
	"github.com/ayanghuang/ayangcache/cache"
	"github.com/ayanghuang/ayangcache/peer"
	"github.com/ayanghuang/ayangcache/singleflight"
	"github.com/ayanghuang/ayangcache/transport"
	"log"
)

type Getter interface {
	// Get 从本地数据源获取数据（例如mysql)
	Get(key string) (byteview.ByteView, error)
}

// GetterFunc  是一个实现了接口的函数类型，简称为接口型函数。
// 作用：既能够将普通的函数类型（需类型转换）作为参数，
// 也可以将结构体作为参数，使用更为灵活，可读性也更好，这就是接口型函数的价值。
type GetterFunc func(key string) (byteview.ByteView, error)

func (f GetterFunc) Get(key string) (byteview.ByteView, error) {
	return f(key)
}

type Group struct {
	addr string
	// 从数据源取出缓存没有的数据
	getter Getter
	// 从缓存中获取数据
	cache cache.Cache
	// 获取远程节点信息
	peers peer.Peer
	// 即是客户端（内部又有服务端），负责发送请求
	client transport.Transport
	// 防止缓存失效
	loads singleflight.Group
}

// NewGroup numCount 为计数器的数量，建议为存储 item 的 10 倍，maxBytes 为最大字节数
func NewGroup(addr string, getter Getter, numCount, maxBytes int64, codecType string) *Group {
	if getter == nil {
		panic("nil Getter")
	}

	group := &Group{
		addr:   addr,
		getter: getter,
		cache:  cache.NewCache(numCount, maxBytes),
		peers:  peer.NewPeer(addr),
		loads:  singleflight.NewGroup(),
	}

	// 通过闭包来捕获当前 Group，传递给下一层依赖。
	getValueFunc := func() transport.GetValueFunc {
		return func(key string) (byteview.ByteView, error) {
			return group.Get(key)
		}
	}
	group.client = transport.NewTransport(addr, codecType, getValueFunc())

	return group
}

func (g *Group) Get(key string) (byteview.ByteView, error) {
	if key == "" {
		return byteview.ByteView{}, fmt.Errorf("key is required")
	}

	// 从缓存中获取
	if v, ok := g.cache.Get(key); ok {

		log.Println(g.addr, "hit cache", "key:", key, "value", v.(byteview.ByteView).String())

		return v.(byteview.ByteView), nil
	}

	return g.load(key)
}

// load
// 1.先判断是否应该从远程结点获取，
// 2. 1. 是，从远程结点获取，获取失败再尝试从数据源获取
// 2. 2. 否，直接从数据源获取
func (g *Group) load(key string) (byteview.ByteView, error) {
	var err error
	value, err := g.loads.Do(key, func() (interface{}, error) {

		// 再次尝试从缓存中取（原因：判断本地没有和从缓存中取不是原子的，从远程获取后会尝试放入本地缓存）
		if val, ok := g.cache.Get(key); ok {

			log.Println(g.addr, "hit cache", "key:", key, "value", val.(byteview.ByteView).String())

			return val, nil
		}

		if g.peers != nil {
			// 获取发送的远程节点
			if peerAddr := g.peers.GetPeer(key); peerAddr != "" {
				// 从远程节点获取
				bytes, err := g.client.GetFromPeer(peerAddr, key)
				if err == nil {

					log.Println(g.addr, "get from peer", peerAddr, "key:", key, "value:", string(bytes))

					// 尝试加入本地缓存
					// 应该设置本地和远程节点缓存的空间比例，而且还应该设置判断是否为 hotkey
					// 这里简单实现了异地缓存，只是利用了缓存的准入原则，且没有空间比例
					val := byteview.NewByteView(bytes)
					g.populateCache(key, val)
					return val, nil
				}
			}
		}

		// 从数据源获取并尝试加入缓存
		val, err := g.getLocally(key)
		if err != nil {
			return byteview.ByteView{}, err
		}
		return val, nil
	})

	if err == nil {
		return value.(byteview.ByteView), nil
	}

	return byteview.ByteView{}, err
}

func (g *Group) populateCache(key string, value byteview.ByteView) {
	g.cache.Add(key, value, int64(value.Len()))
}

// 从本地数据源获取数据
func (g *Group) getLocally(key string) (byteview.ByteView, error) {
	// 从数据源获取
	val, err := g.getter.Get(key)
	if err != nil {
		return byteview.ByteView{}, err
	}

	log.Println(g.addr, "get data from dataSource", "key:", key, "value:", val)

	// 尝试加入缓存中
	g.populateCache(key, val)
	return val, nil
}

func (g *Group) RegisterPeers(addr ...string) {
	g.peers.RegisterPeers(addr...)
}
