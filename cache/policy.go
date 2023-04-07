package cache

type policy interface {
	// DealGet 负责在缓冲区耗尽时批量接收和处理因为获取缓存导致键对应的 fre 需要更改等操作
	DealGet([]uint64)
	// Add 把新的 key 加入缓存，如满足准入策略且缓存已满，则选择一部分需要淘汰的 key 返回
	Add(uint64, int64) ([]*item, bool)
	// Del 删除缓存
	Del(uint64)
}
