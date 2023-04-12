package cache

import "sync"

type ringBuffer interface {
	// Push 加入缓存池中，如果满了（批量）就发送到 policy 的 itemCh（chan）中由异步协程接受处理
	Push(uint64)
}

type consumer interface {
	ConsumeGet([]uint64) bool
}

type ringBufferPool struct {
	pools sync.Pool
}

func newRingBufferPool(c consumer, cap int) *ringBufferPool {
	if c == nil || cap == 0 {
		panic("dealGet is nil or cap is 0")
	}

	return &ringBufferPool{
		pools: sync.Pool{
			New: func() any {
				return newRingStore(c, cap)
			},
		},
	}
}

func (rb *ringBufferPool) Push(hashKey uint64) {
	rs := rb.pools.Get().(*ringStore)
	rs.push(hashKey)
	rb.pools.Put(rs)
}

type ringStore struct {
	// 到时候传入 policy，切片满了就调用 consumer.ConsumeGet 交给 policy 接收处理
	// 为什么不直接用 policy，而是重新定义了 consumer 接口呢？
	// 接口越小越好。只需要用到 policy 接口中的一个方法，就重新声明了一个小接口，隐藏了其他方法，防止胡乱调用
	consumer
	hashKeys []uint64
	// cap 建议 64
	cap int
}

func newRingStore(c consumer, cap int) *ringStore {
	return &ringStore{
		consumer: c,
		hashKeys: make([]uint64, 0, cap),
		cap:      cap,
	}
}

func (rs *ringStore) push(hashKey uint64) {
	rs.hashKeys = append(rs.hashKeys, hashKey)

	// 已满需要批量处理了，发送给 policy
	if len(rs.hashKeys) >= rs.cap {
		// 没处理就被丢弃了，直接复用数组
		if ok := rs.ConsumeGet(rs.hashKeys); !ok {
			rs.hashKeys = rs.hashKeys[:0]
		} else {
			rs.hashKeys = make([]uint64, 0, rs.cap)
		}
	}
}
