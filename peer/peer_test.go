package peer

import (
	"testing"
	"time"
)

// TestPeer 测试 hash 功能是否正确，测试一致性 hash 是否有效
func TestPeer(t *testing.T) {
	// 多次重复执行，先执行命令，后执行测试函数，可以看到多次结果一致，则结果正确。另外：多加 etcdctl put /ayangcache/node/102  127.0.0.1:3333 一个发现对原结果影响较少，体现一致性 hash 算法的作用
	//etcdctl del --prefix "/ayangcache"
	//etcdctl put /ayangcache/node/100  127.0.0.1:1111
	//etcdctl put /ayangcache/node/101  127.0.0.1:2222
	p := NewPeer("127.0.0.1:8888", "127.0.0.1:2379")
	strs := [5]string{"ayang", "tom", "HQUer", "cache", "ayangcache"}

	for i := range strs {
		if addr := p.GetPeer(strs[i]); addr == "" {
			println("本节点")
		} else {
			println(addr)
		}
	}

	// 测试结果
	//127.0.0.1:2222
	//本节点
	//127.0.0.1:2222
	//127.0.0.1:1111
	//127.0.0.1:1111

	// 多加 127.0.0.1：3333 结果
	//127.0.0.1:2222
	//127.0.0.1:3333
	//127.0.0.1:2222
	//127.0.0.1:3333
	//127.0.0.1:1111
}

// TestPeer2 测试监听，中途增加后删除是否一致
func TestPeer2(t *testing.T) {
	//etcdctl del --prefix "/ayangcache"
	//etcdctl put /ayangcache/node/100  127.0.0.1:1111
	//etcdctl put /ayangcache/node/101  127.0.0.1:2222
	p := NewPeer("127.0.0.1:8888", "127.0.0.1:2379")
	strs := [5]string{"ayang", "tom", "HQUer", "cache", "ayangcache"}

	for j := 0; j < 3; j++ {
		for i := range strs {
			if addr := p.GetPeer(strs[i]); addr == "" {
				println("本节点")
			} else {
				println(addr)
			}
		}
		println("--------")
		time.Sleep(time.Second * 5)
		// 第一次执行下面代码，表示有更新
		//etcdctl put /ayangcache/node/102  127.0.0.1:3333
		// 第二次执行删除
		//etcdctl del /ayangcache/node/102
	}

	// 测试结果
	//127.0.0.1:2222
	//本节点
	//127.0.0.1:2222
	//127.0.0.1:1111
	//127.0.0.1:1111
	//--------
	//127.0.0.1:2222
	//127.0.0.1:3333
	//127.0.0.1:2222
	//127.0.0.1:3333
	//127.0.0.1:1111
	//--------
	//127.0.0.1:2222
	//本节点
	//127.0.0.1:2222
	//127.0.0.1:1111
	//127.0.0.1:1111
	//--------

}
