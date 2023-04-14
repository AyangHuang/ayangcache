package cache

import (
	"math/rand"
	"time"
)

// cmSketch is a Count-Min sketch implementation with 4-bit counters, heavily
// based on Damian Gryski's CM4 [1].
//
// [1]: https://github.com/dgryski/go-tinylfu/blob/master/cm4.go

const (
	cmDepth = 4
)

type cmSketch struct {
	// cmDepth 为四行，也就是有四个布隆过滤器
	rows [cmDepth]cmRow
	// 每个布隆过滤器一个种子，这样一个 hash 就会分布到四行中不同位置，每次四个布隆过滤器中的取最小值就可以了
	seed [cmDepth]uint64
	// 布隆过滤器的实际大小，为（2 的整数次幂-1），这样可以利用位运算进行快速取模
	mask uint64
}

func newCmSketch(numCounters int64) *cmSketch {
	if numCounters == 0 {
		panic("cmSketch: bad numCounters")
	}

	numCounters = next2Power(numCounters)
	sketch := &cmSketch{mask: uint64(numCounters - 1)}
	source := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	for i := 0; i < cmDepth; i++ {
		sketch.seed[i] = source.Uint64()
		sketch.rows[i] = newCmRow(numCounters)
	}
	return sketch
}

func (s *cmSketch) Increment(hashed uint64) {
	for i := range s.rows {
		s.rows[i].increment((hashed ^ s.seed[i]) & s.mask)
	}
}

// Estimate 获取频率
func (s *cmSketch) Estimate(hashed uint64) int {
	min := byte(255)
	for i := range s.rows {
		val := s.rows[i].get((hashed ^ s.seed[i]) & s.mask)
		if val < min {
			min = val
		}
	}
	return int(min)
}

// Reset 保持鲜度，全部减半
func (s *cmSketch) Reset() {
	for _, r := range s.rows {
		r.reset()
	}
}

func (s *cmSketch) Clear() {
	for _, r := range s.rows {
		r.clear()
	}
}

type cmRow []byte

func newCmRow(numCounters int64) cmRow {
	// / 2 是因为一个 byte 可以存2 个 4 bit 的记录
	return make(cmRow, numCounters/2)
}

func (r cmRow) get(n uint64) byte {
	// n 是第几个，n / 2 是换成布隆中的第几个 byte
	// >> (n&1)*4 表示如果 n%2 有余数，也就是选中的 byte 是 aaaa0000，aaaa 才是想要的，所以要位移 4 位
	// 具体为什么一个 byte 是这样存，因为 increment 中我们规定这样存，实际我们也可以规定方向反着存，这样 n%2没有余数我们才需要位移 4 位
	// & 15 即 & 01111 取后 4 个有效位
	return byte(r[n/2]>>((n&1)*4)) & 0x0f
}

// increment 看 get 解释
func (r cmRow) increment(n uint64) {
	i := n / 2
	s := (n & 1) * 4
	v := (r[i] >> s) & 0x0f
	if v < 15 {
		r[i] += 1 << s
	}
}

func (r cmRow) reset() {
	for i := range r {
		r[i] = (r[i] >> 1) & 0x77
	}
}

func (r cmRow) clear() {
	for i := range r {
		r[i] = 0
	}
}

// next2Power 对 x 向上舍入得到最近的 2 的整数次幂
func next2Power(x int64) int64 {
	x--
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	x |= x >> 32
	x++
	return x
}
