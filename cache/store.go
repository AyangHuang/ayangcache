package cache

import (
	"sync"
	"time"
)

const (
	concurrentMapSize = 256
)

type store interface {
	Get(uint64, uint64) (interface{}, bool)
	Add(uint64, uint64, interface{}, time.Time) bool
	Del(uint64, uint64) (interface{}, bool)
}

type storeItem struct {
	hashKey    uint64
	conflict   uint64
	value      interface{}
	expiration time.Time
}

// shareStore 使用分段锁实现，即 256 个 map 分别拥有锁，以确保对 shareStore 尽可能高的并行访问
type shareStore struct {
	store [concurrentMapSize]*concurrentMap
}

func newShareStore() *shareStore {
	s := &shareStore{}

	for i := 0; i < concurrentMapSize; i++ {
		s.store[i] = new(concurrentMap)
		s.store[i].date = make(map[uint64]*storeItem)
	}

	return s
}

func (s *shareStore) Get(hashKey, conflict uint64) (interface{}, bool) {
	return s.store[hashKey%concurrentMapSize].get(hashKey, conflict)
}

func (s *shareStore) Add(hashKey, conflict uint64, value interface{}, expiration time.Time) bool {
	return s.store[hashKey%concurrentMapSize].add(hashKey, conflict, value, expiration)
}

func (s *shareStore) Del(hashKey, conflict uint64) (interface{}, bool) {
	return s.store[hashKey%concurrentMapSize].del(hashKey, conflict)
}

type concurrentMap struct {
	// mutex 不采用匿名引入，因为 Lock 和 Unlock 方法不需要暴露出来
	// 同时在方法内部调用 Lock，使得方法是并发安全的
	mutex sync.Mutex
	date  map[uint64]*storeItem
}

func (m *concurrentMap) get(hashKey, conflict uint64) (interface{}, bool) {
	m.mutex.Lock()

	// 存在
	if item, ok := m.date[hashKey]; ok && item.conflict == conflict {
		// 且未超时
		if item.expiration.IsZero() || item.expiration.After(time.Now()) {
			m.mutex.Unlock()
			return item.value, ok
		}
	}

	m.mutex.Unlock()
	return nil, false
}

func (m *concurrentMap) add(hashKey, conflict uint64, value interface{}, expiration time.Time) bool {
	m.mutex.Lock()

	if !expiration.IsZero() && expiration.Before(time.Now()) {
		m.mutex.Unlock()
		return false
	}

	// hashKey 已存在，当然可能 conflict 不相等，但这种情况是不能存进去的，map 的 key 不允许重复，会覆盖原来的 key
	// hashKey 已存在，但是过期了，直接覆盖存进去就可以了
	if item, ok := m.date[hashKey]; ok {
		if item.expiration.IsZero() || item.expiration.After(time.Now()) {
			m.mutex.Unlock()
			return false
		}
	}

	m.date[hashKey] = &storeItem{
		hashKey:    hashKey,
		conflict:   conflict,
		value:      value,
		expiration: expiration,
	}

	m.mutex.Unlock()
	return true
}

func (m *concurrentMap) del(hashKey, conflict uint64) (interface{}, bool) {
	m.mutex.Lock()

	item, ok := m.date[hashKey]

	// 不存在或者 conflict 不相等(conflict != 0 表示不用看 conflict)
	// 因为 policy 只存 hashKey，不存 conflict，所以从 policy 淘汰只需要用到 key
	if !ok || (item.conflict != 0 && item.conflict != conflict) {
		m.mutex.Unlock()
		return nil, false
	}

	delete(m.date, hashKey)

	m.mutex.Unlock()
	return item.value, true
}
