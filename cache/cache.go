package cache

import "time"

type Cache interface {
	Get(key interface{}) (interface{}, bool)
	Add(key, val interface{}, cost int64) bool
	AddWithTTL(key, value interface{}, cost int64, ttl time.Duration) bool
}

// item 整合成一个 struct，方便函数传参
type item struct {
	hashKey    uint64
	conflict   uint64
	value      interface{}
	cost       int64
	expiration time.Time
}

type cache struct {
	// 存储所有完整的（key，value）
	store store
	// 缓存淘汰和准入策略，与上面解耦，所以也会存储所有的 key
	policy policy
	// 需要增加缓存丢入这个 chan，协程异步处理
	addBuf chan *item
	// 获取缓存后，需要修改缓存获取频率（LFU）或移到队首（LRU）等操作，直接丢入这个 buffer，有异步协程调用 policy 提供的接口处理
	getBuf ringBuffer
	// 循环定时器，每隔一段时间触发并调用 expiration.Clean 扫描过去一段时间过期的 key 并清除
	cleanupTicker *time.Ticker
	// 按过期时间分桶存储 key，定期删除一部分 key
	expiration expiration
}

func (c *cache) Get(key interface{}) (interface{}, bool) {
	if key == nil {
		return nil, false
	}

	hashKey, conflict := KeyToHash(key)

	// 获取缓存，所以需要增加该键的频率
	// 注意：不管该键存不存在缓存中，都需要增加。因为有准入策略。
	c.getBuf.Push(hashKey)

	if v, ok := c.store.Get(hashKey, conflict); ok {
		return v, true
	}

	return nil, false
}

func (c *cache) Add(key, value interface{}, cost int64) bool {
	return c.AddWithTTL(key, value, cost, 0*time.Second)
}

func (c *cache) AddWithTTL(key, value interface{}, cost int64, ttl time.Duration) bool {
	if key == nil || value != nil || cost == 0 {
		return false
	}

	var expiration time.Time
	switch {
	case ttl == 0:
		// 没有过期时间，后期用 time.IsZero 判断即可
		break
	case ttl < 0:
		return false
	default:
		expiration = time.Now().Add(ttl)
	}

	hashKey, conflict := KeyToHash(key)
	i := &item{
		hashKey:    hashKey,
		conflict:   conflict,
		cost:       cost,
		value:      value,
		expiration: expiration,
	}

	select {
	case c.addBuf <- i:
		return true
	// 抗争用，如果 addBuf 实在没空闲位置了，表示处理不过来，那么不要阻塞等待，直接放弃加入
	// 这样会造成缓存丢失，但可以提高 api 速度和抗争用性
	default:
	}

	return false
}
