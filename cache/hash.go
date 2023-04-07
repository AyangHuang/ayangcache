package cache

import (
	"github.com/cespare/xxhash/v2"
	"unsafe"
)

type stringStruct struct {
	str unsafe.Pointer
	len int
}

//go:noescape
//go:linkname memhash runtime.memhash
func memhash(p unsafe.Pointer, h, s uintptr) uintptr

// MemHash 是 map 使用的 hash 函数，利用硬件指令，速度快
// 该 hash 函数使用的种子在进程初始化的时候初始化，每个进程都不相同，所以不能用作持久化哈希
func memHash(data []byte) uint64 {
	ss := (*stringStruct)(unsafe.Pointer(&data))
	return uint64(memhash(ss.str, 0, uintptr(ss.len)))
}

func memHashString(str string) uint64 {
	ss := (*stringStruct)(unsafe.Pointer(&str))
	return uint64(memhash(ss.str, 0, uintptr(ss.len)))
}

// KeyToHash 返回两个 hash 结果，分别用两个不同的 hash 函数，可以避免 hash 冲突
// 为什么不直接把 key 作为 map 的 key？
// 因为 store 和 policy 中的 map 都需要用到 key，这样如果 key 很大，会造成很大的冗余成本。
func KeyToHash(key interface{}) (uint64, uint64) {
	if key == nil {
		return 0, 0
	}
	switch k := key.(type) {
	case uint64:
		return k, 0
	case string:
		return memHashString(k), xxhash.Sum64String(k)
	case []byte:
		return memHash(k), xxhash.Sum64(k)
	case byte:
		return uint64(k), 0
	case int:
		return uint64(k), 0
	case int32:
		return uint64(k), 0
	case uint32:
		return uint64(k), 0
	case int64:
		return uint64(k), 0
	default:
		panic("Key type not supported")
	}
}
