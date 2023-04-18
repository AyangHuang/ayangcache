package transport

import (
	"context"
	"errors"
	"time"
)

const (
	// 10 秒
	sendTimeOutMicrosecond = 10000
)

type Transport interface {
	GetFromPeer(addr string, key string) ([]byte, error)
}

type transport struct {
	client *client
	server *server
	// 获取编码的方式
	codec NewCodecFunc
}

func NewTransport(addr string, codecType string, valueFunc GetValueFunc) Transport {
	codecFunc, ok := codecMap[codecType]
	if !ok {
		panic("error codecType")
	}

	t := &transport{
		client: newClient(codecFunc),
		codec:  codecFunc,
		server: newServer(addr, codecFunc, valueFunc),
	}

	// 开启服务器服务
	go t.server.Serve()

	return t
}

func (t *transport) GetFromPeer(addr string, key string) ([]byte, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond*sendTimeOutMicrosecond)
	// 使得等待在上面的返回，和后面 peerConn.send 对应
	defer cancel()

	call := &call{
		addr: addr,
		RequestBody: &RequestBody{
			key: key,
		},
		valCh:   make(chan []byte),
		timeout: timeoutCtx,
	}

	go t.client.send(call)

	// 上面是开启一个子协程去发送（异步），这里又阻塞等待结果，不是又同步了么？
	// 确实是同步的，不过同步的只有最多 timeout 的时间，过了这个时间会立刻返回（那个子协程此时就是异步执行了）
	// 如果不采取子协程异步发送，那么可能会一直阻塞在 send 方法很久
	// 也就是说实现超时立刻返回功能必须是异步的
	select {
	case <-call.timeout.Done():
		return nil, errors.New("get from peers timeout")
	case val := <-call.valCh:
		if call.err != nil {
			return nil, call.err
		}
		return val, nil
	}
}

type call struct {
	addr string
	*RequestBody
	// 通过 chan 与子 goroutine 通信拿到结果
	valCh chan []byte
	// 发送的总超时（简单一点：是总超时，就不细分其他超时时间了）
	timeout context.Context
	err     error
}
