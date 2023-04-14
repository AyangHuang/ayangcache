package cache

import (
	"fmt"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	fn := OptionRingBufferSize(1)
	// 加大计数器，使得非常精准
	c := NewCache(4*10*10, 4, fn)

	fmt.Println("1 part")

	// part 1
	for i := 0; i < 4; i++ {
		go c.Add(i, i, 1)
	}

	time.Sleep(time.Second)

	for i := 0; i < 4; i++ {
		i := i
		go func() {
			get, ok := c.Get(i)
			if ok {
				fmt.Println(get.(int)) // 0 1 2 3
			}
		}()
	}

	// part 2

	time.Sleep(time.Second)
	c.Add(100, 100, 1)
	time.Sleep(time.Second)

	fmt.Println("2 part")
	for i := 0; i < 5; i++ {
		i := i
		go func() {
			get, ok := c.Get(i)
			if ok {
				fmt.Println(get.(int)) // 一定：0 1 2 3。因为 4 由于准入策略加不入
			}
		}()
	}

	// part 3

	time.Sleep(time.Second)
	c.Get(4)
	c.Get(4)
	time.Sleep(time.Second)
	c.Add(4, 4, 2)
	time.Sleep(time.Second)

	fmt.Println("3 part")

	for i := 0; i < 5; i++ {
		i := i
		go func() {
			get, ok := c.Get(i)
			if ok {
				fmt.Println(get.(int)) // 必定有 4，其实 2 个 任意：0 1 2 3
			}
		}()
	}

	time.Sleep(time.Second)
}
