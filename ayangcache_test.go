package ayangcache

import (
	"errors"
	"fmt"
	"github.com/ayanghuang/ayangcache/byteview"
	"github.com/ayanghuang/ayangcache/transport"
	"testing"
	"time"
)

var dataSource *mockDataSource

func init() {
	dataSource = &mockDataSource{
		m: make(map[string]byteview.ByteView),
	}
	dataSource.m["ayang"] = byteview.NewByteView([]byte("ayangValue"))
	dataSource.m["tom"] = byteview.NewByteView([]byte("tomValue"))
	dataSource.m["ayangcache"] = byteview.NewByteView([]byte("ayangcacheValue"))
}

type mockDataSource struct {
	m map[string]byteview.ByteView
}

func (source *mockDataSource) Get(key string) (byteview.ByteView, error) {
	v, ok := source.m[key]
	if !ok {
		return byteview.ByteView{}, errors.New("no this cache")
	}
	return v, nil
}

func TestGroup_Get(t *testing.T) {
	// etcdctl del --prefix "/ayangcache"
	etcdEndPoint := "127.0.0.1:2379"
	g1Addr, g2Addr, g3Addr := "127.0.0.1:5555", "127.0.0.1:6666", "127.0.0.1:7777"

	g1 := NewGroup(g1Addr, etcdEndPoint, dataSource, 2<<10, 2<<10, transport.ProtobufType)
	g2 := NewGroup(g2Addr, etcdEndPoint, dataSource, 2<<10, 2<<10, transport.ProtobufType)
	_ = NewGroup(g3Addr, etcdEndPoint, dataSource, 2<<10, 2<<10, transport.ProtobufType)

	var err error

	v, err := g2.Get("tom")
	if err != nil {
		fmt.Println("-------------------", "err:", err.Error())
	} else {
		fmt.Println("-------------------", "value:", v.String())
	}

	v, err = g1.Get("ayang")
	if err != nil {
		fmt.Println("-------------------", "err:", err.Error())
	} else {
		fmt.Println("-------------------", "value:", v.String())
	}

	v, err = g2.Get("ayangcache")
	if err != nil {
		fmt.Println("-------------------", "err:", err.Error())
	} else {
		fmt.Println("-------------------", "value:", v.String())
	}

	v, err = g1.Get("nocache")
	if err != nil {
		fmt.Println("-------------------", "err:", err.Error())
	} else {
		fmt.Println("-------------------", "value:", v.String())
	}

	v, err = g1.Get("ayang")
	if err != nil {
		fmt.Println("-------------------", "err:", err.Error())
	} else {
		fmt.Println("-------------------", "value:", v.String())
	}

	v, err = g2.Get("ayang")
	if err != nil {
		fmt.Println("-------------------", "err:", err.Error())
	} else {
		fmt.Println("-------------------", "value:", v.String())
	}

	time.Sleep(time.Second)
	return
}
