package cache

import (
	"sync"
	"time"
)

const (
	// secondsPerBucket 该大小为时间周期，同一过期时间周期处于同一个 bucket
	secondsPerBucket = int64(5)
)

func expirationBucket(time time.Time) int64 {
	return time.Unix()/secondsPerBucket + 1
}

func cleanBucket() int64 {
	// 清除 1 个 secondsPerBucket 之前过期的 key
	return time.Now().Unix()/secondsPerBucket - 1
}

type expiration interface {
	Add(uint64, uint64, time.Time)
	// Clean 通过函数传参依赖于 store 和 policy（设计模式中的依赖关系），还有另外 2 种解决方法
	// 2. store 和 policy 通过 newExpirationMap 时传入并作为内置的属性（设计模式中的关联关系，属于强依赖），后面通过 field.Method 调用
	// 3. Clean() 既不需要属性依赖也不需要参数依赖，直接返回需要删除的 key，由外部 cache 调用 Clean 时接收然后再调用 store.Del 和 policy.Del
	// 很明显我认为第 3 种是最好的，无依赖，且单一责职。先记录下，后面再来修改把。
	Clean(store, policy)
}

// bucket 桶，处于同一个桶的是同一个 secondsPerBucket 秒过期周期的 key
// 之所以用 slice 而不用 map，因为不允许更新，也就不需要快速查找替换功能，所有选用 slice 比 map 节省空间
type bucket []itemPair

type expirationMap struct {
	// mutex 不采用匿名引入，因为 Lock 和 Unlock 方法不需要暴露出来
	// 同时在方法内部调用 Lock，使得方法是并发安全的
	mutex   sync.Mutex
	buckets map[int64]*bucket
}

type itemPair struct {
	hashKey  uint64
	conflict uint64
}

func newExpirationMap() *expirationMap {
	return &expirationMap{
		buckets: make(map[int64]*bucket),
	}
}

func (em *expirationMap) Add(hashKey, conflict uint64, expiration time.Time) {
	if expiration.IsZero() {
		return
	}

	// 以过期时间所在的时间周期为 map 的 key 值
	mapKey := expirationBucket(expiration)

	em.mutex.Lock()
	defer em.mutex.Unlock()

	s, ok := em.buckets[mapKey]
	if !ok {
		// 不存在该 bucket，说明是该 secondsPerBucket 周期内的第一个 key
		s = new(bucket)
		em.buckets[mapKey] = s
	}

	*s = append(*s, itemPair{hashKey: hashKey, conflict: conflict})
}

func (em *expirationMap) Clean(store store, policy policy) {
	// 获取需要删除的周期 key
	mapKey := cleanBucket()

	em.mutex.Lock()

	s, ok := em.buckets[mapKey]
	if !ok {
		em.mutex.Unlock()
		return
	}

	// 删除时间周期桶
	delete(em.buckets, mapKey)
	em.mutex.Unlock()

	// 根据桶中的 key 去 store 和 policy 中删除
	for i := 0; i < len(*s); i++ {
		store.Del((*s)[i].hashKey, (*s)[i].conflict)
		policy.Del((*s)[i].hashKey)
	}
}
