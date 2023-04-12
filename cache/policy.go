package cache

type policy interface {
	// consumer.ConsumeGet 负责在缓冲区耗尽时批量接收并发送到 policy 的接受 chan 中处理
	consumer
	// Add 把新的 key 加入缓存，如满足准入策略且缓存已满，则选择一部分需要淘汰的 key 返回
	Add(uint64, int64) ([]*item, bool)
	// Del 删除缓存
	Del(uint64)
}

type defaultPolicy struct {
	itemChan chan []uint64
}

func (policy *defaultPolicy) ConsumeGet(hashKeys []uint64) bool {
	select {
	case policy.itemChan <- hashKeys:
		return true
	default:
		// 抗争用，如果缓冲区满了，不要阻塞直接丢弃
		return false
	}
}
