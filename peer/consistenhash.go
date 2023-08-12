package peer

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

type Map struct {
	// hash函数
	hash Hash
	// 一个真实结点对应多少个虚拟结点
	replaces int
	// 虚拟结点环，存储所有的虚拟节点。排序，从小到大，从0-2^32-1
	virtualRing []int
	// 根据虚拟结点找到真实结点。map[int 虚拟节点](string ip:port 真实结点）
	hashMap map[int]string
}

func NewMap(replaces int, hash Hash) *Map {
	m := &Map{
		hash:     hash,
		replaces: replaces,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Init 为什么每次更新节点（加入或删除）都要重新初始化呢？因为要保证所有节点的 hash 环都一致，否则可能因为先后顺序产生 hash 冲突不一致
func (m *Map) Init(realNode ...string) {
	newVirtualRing := make([]int, 0, len(realNode))
	newHashMap := make(map[int]string, len(realNode)*m.replaces*2)

	for _, v := range realNode {
		// 1 个真实的节点对应 replace 个虚拟节点
		for i := 1; i <= m.replaces; i++ {
			for j := 1; ; j++ {
				// 虚拟节点
				hash := int(m.hash([]byte(strconv.Itoa(i*j) + v)))
				_, ok := newHashMap[hash]
				if !ok {
					// 虚拟节点存入环
					newVirtualRing = append(newVirtualRing, hash)
					// 存储虚拟节点和真实节点的映射
					newHashMap[hash] = v
					break
				}
				// 已经存在，hash 冲突，继续下次尝试
			}

		}
	}

	sort.Ints(newVirtualRing)
	m.virtualRing = newVirtualRing
	m.hashMap = newHashMap
}

// Get 根据 key 值选择合适的节点
func (m *Map) Get(key string) string {
	hash := int(m.hash([]byte(key)))

	// 二分查找第一个大于或等于的虚拟节点
	idx := sort.Search(len(m.virtualRing), func(i int) bool { return m.virtualRing[i] >= hash })

	// 找不到，则对应第一个
	if idx == len(m.virtualRing) {
		idx = 0
	}

	if addr, ok := m.hashMap[m.virtualRing[idx]]; ok {
		return addr
	} else {
		return ""
	}

}
