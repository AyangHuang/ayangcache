package cache

import (
	"math"
	"sync"
)

type policy interface {
	// consumer.ConsumeGet 负责在缓冲区耗尽时批量接收并发送到 policy 的接受 chan 中处理
	consumer
	// Add 把新的 key 加入缓存，如满足准入策略且缓存已满，则选择一部分需要淘汰的 key 返回
	Add(uint64, int64) ([]uint64, bool)
	// Del 删除缓存
	Del(uint64)
}

const (
	samplelfu    = 5
	itemChanSize = 3
)

type defaultPolicy struct {
	mutex sync.Mutex
	admit *tinyLFU
	// sampledLFU 近似 LFU，每次要删除从这里随机取出几个值和要加入进行比较
	// 存储所有的 key 和 cost，为什么？
	// 其实是为了和 store 解耦，两方互不干扰。确实是冗余了。
	// 如果不想冗余，就直接让 store 暴露一个随机获取的方法，然后 policy 调用即可。但是双方方会耦合在一起。
	evict    *sampledLFU
	itemChan chan []uint64
}

func newDefaultPolicy(numCount int64, maxCost int64) *defaultPolicy {
	newPolicy := &defaultPolicy{
		// numCount 是计数器的容量，建议为该缓存中总容量的的 10 倍
		admit: newTinyLFU(numCount),
		evict: newSampledFlU(maxCost),
		// 为什么才 3？ristretto 解释说避免消耗太多的 CPU 来处理？？？
		itemChan: make(chan []uint64, itemChanSize),
	}

	// 守护协程异步批量处理
	go newPolicy.processItems()

	return newPolicy
}

func (policy *defaultPolicy) processItems() {
	for {
		select {
		case hashKeys := <-policy.itemChan:
			policy.mutex.Lock()
			for i := range hashKeys {
				policy.admit.incrementFre(hashKeys[i])
			}
			policy.mutex.Unlock()
		}
	}
}

// ConsumeGet 例外，不需要上锁，由 chan 内部保证
func (policy *defaultPolicy) ConsumeGet(hashKeys []uint64) bool {
	select {
	case policy.itemChan <- hashKeys:
		return true
	default:
		// 抗争用，如果缓冲区满了，不要阻塞直接丢弃
		return false
	}
}

// Add 根据准入策略放行，返回因加入后容量不足需要淘汰的 item
func (policy *defaultPolicy) Add(hashKey uint64, cost int64) ([]uint64, bool) {
	policy.mutex.Lock()
	defer policy.mutex.Unlock()

	if cost > policy.evict.getMaxCost() {
		return nil, false
	}

	// 已存在，因为并不提供修改缓存的功能，所以直接返回
	if _, ok := policy.evict.getCost(hashKey); ok {
		return nil, false
	}

	// 加入后还剩下多少
	remainRom := policy.evict.remainRoom(cost)
	// 直接加入
	if remainRom >= 0 {
		policy.evict.add(hashKey, cost)
		return nil, true
	}

	addItemFre := policy.admit.getFrequent(hashKey)
	sampleItems := make([]keyPair, 0, samplelfu)
	var out []uint64

	for remainRom < 0 {
		policy.evict.fillSample(&sampleItems)

		// 遍历找到最小的
		var minFre = math.MaxInt
		var minKeyPair keyPair
		var minIndex int
		for index := range sampleItems {
			fre := policy.admit.getFrequent(sampleItems[index].hashKey)
			if fre < minFre {
				minFre = fre
				minIndex = index
				minKeyPair.hashKey = sampleItems[index].hashKey
				minKeyPair.cost = sampleItems[index].cost
			}
		}

		// 不符合准入策略
		if minFre > addItemFre {
			return out, false
		}

		// 把最小的踢出去 sample
		sampleItems[minIndex] = sampleItems[len(sampleItems)-1]
		sampleItems = sampleItems[:len(sampleItems)-1]
		// 加入淘汰 slice
		out = append(out, minKeyPair.hashKey)

		// 从 policy 中删除
		policy.evict.del(minKeyPair.hashKey)
		// 增加剩余空间
		remainRom += minKeyPair.cost
	}

	// 加入 policy
	policy.evict.add(hashKey, cost)

	return out, true
}

func (policy *defaultPolicy) Del(hashKey uint64) {
	policy.mutex.Lock()
	policy.evict.del(hashKey)
	policy.mutex.Unlock()
}

type tinyLFU struct {
	fre   *cmSketch
	incrs int64
	// incrs 到达 reset 后需要进行保鲜，所有 fre 减半
	reset int64
}

func newTinyLFU(numCount int64) *tinyLFU {
	return &tinyLFU{
		fre:   newCmSketch(numCount),
		reset: numCount,
	}
}

func (tinyLFU *tinyLFU) incrementFre(hashKey uint64) {
	tinyLFU.fre.Increment(hashKey)

	tinyLFU.incrs++
	// 保鲜，全部频率减半
	if tinyLFU.incrs == tinyLFU.reset {
		tinyLFU.fre.Reset()
		tinyLFU.incrs = 0
		tinyLFU.reset = 0
	}
}

func (tinyLFU *tinyLFU) getFrequent(hashKey uint64) int {
	return tinyLFU.fre.Estimate(hashKey)
}

type sampledLFU struct {
	maxCost  int64
	used     int64
	keyCosts map[uint64]int64
}

type keyPair struct {
	hashKey uint64
	cost    int64
}

func newSampledFlU(maxCost int64) *sampledLFU {
	return &sampledLFU{
		maxCost:  maxCost,
		keyCosts: make(map[uint64]int64),
	}
}

func (sampledLFU *sampledLFU) getMaxCost() int64 {
	return sampledLFU.maxCost
}

func (sampledLFU *sampledLFU) getCost(hashKey uint64) (cost int64, ok bool) {
	cost, ok = sampledLFU.keyCosts[hashKey]
	return
}

func (sampledLFU *sampledLFU) remainRoom(cost int64) int64 {
	return sampledLFU.maxCost - sampledLFU.used - cost
}

func (sampledLFU *sampledLFU) add(hashKey uint64, cost int64) {
	sampledLFU.keyCosts[hashKey] = cost
	sampledLFU.used += cost
}

func (sampledLFU *sampledLFU) del(hashKey uint64) {
	if cost, ok := sampledLFU.getCost(hashKey); ok {
		delete(sampledLFU.keyCosts, hashKey)
		sampledLFU.used -= cost
	}
}

func (sampledLFU *sampledLFU) fillSample(sampleItems *[]keyPair) {
	// 利用 map 随机遍历的特性
	for hashKey, cost := range sampledLFU.keyCosts {
		*sampleItems = append(*sampleItems, keyPair{
			hashKey: hashKey,
			cost:    cost})

		if len(*sampleItems) >= samplelfu {
			return
		}
	}
}
