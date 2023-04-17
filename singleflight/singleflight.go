package singleflight

import (
	"sync"
)

type Group interface {
	Do(key string, fn func() (interface{}, error)) (interface{}, error)
}

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type group struct {
	sync.Mutex
	sync.Once
	m map[string]*call
}

func NewGroup() Group {
	return &group{
		m: make(map[string]*call),
	}
}

func (g *group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 懒加载且只调用一次，即单例
	g.Once.Do(func() {
		g.m = make(map[string]*call)
	})

	g.Lock()

	if c, ok := g.m[key]; ok {
		// 解锁，让其他并发的请求可以进来
		g.Unlock()
		// 等待get获取
		c.wg.Wait()
		return c.val, c.err
	}

	c := &call{}
	g.m[key] = c
	// 让等待
	c.wg.Add(1)

	// 解锁让其他并发的进去等待
	g.Unlock()

	c.val, c.err = fn()
	// 让等待的其他并发获取数据
	c.wg.Done()

	g.Lock()
	//删除key，并发结束
	delete(g.m, key)
	g.Unlock()

	return c.val, c.err
}
