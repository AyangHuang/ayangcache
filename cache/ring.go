package cache

type ringBuffer interface {
	// Push 加入缓存池中，如果满了（批量）就发送到 policy 的 itemCh（chan）中由异步协程接受处理
	Push(uint64)
}
