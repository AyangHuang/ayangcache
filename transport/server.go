package transport

import (
	"ayangcache/byteview"
	"github.com/panjf2000/ants/v2"
	"log"
	"net"
	"sync"
	"time"
)

const (
	goroutinePoolSize = 4096
)

var gPool *ants.Pool

func init() {
	var err error
	gPool, err = ants.NewPool(goroutinePoolSize)
	if err != nil {
		panic("init goroutine pool fail")
	}
}

// GetValueFunc 做法一
type GetValueFunc func(string) (byteview.ByteView, error)

// 做法二：在本包增加一个 Get(key string) (byteview.ByteView, error)（为什么不直接用 ayangcache 包的接口，还要造一个新的接口，因为会造成循环依赖）
// 然后在 server 创建时把 Group 传入作为 server 的 file（该字段的类型是具有 Get 方法的接口）
//type GetValueFunc interface {
//	Get(key string) (byteview.ByteView, error)
//}

type server struct {
	// 本节点地址
	addr string
	// 编码格式
	codec NewCodecFunc
	// 从本节点获取缓存
	getValueFunc GetValueFunc
}

func newServer(addr string, codec NewCodecFunc, getValueFunc GetValueFunc) *server {
	server := &server{
		addr:         addr,
		codec:        codec,
		getValueFunc: getValueFunc,
	}
	return server
}

func (s *server) Serve() {
	server, err := net.Listen("tcp", s.addr)
	defer func() {
		_ = server.Close()
	}()

	if err != nil {
		panic("server start fail")
	}

	for {
		var err error
		// 等待客户端连接
		conn, err := server.Accept()
		if err != nil {
			log.Println("server err:", err.Error())
			return
		}

		_ = newClientConn(conn, s)

	}
}

type clientConn struct {
	server *server
	// 真正的 TCP 连接
	conn net.Conn
	// 编码方式，其实就是嵌套读写 conn
	codec ServerCodec
	// 发送到 writeLoop 中处理
	writeCh chan *ResponseBody
	closeDo sync.Once
	// 关闭信号，采用直接 close chan 的方式
	// close chan 会：
	// 1. 导致阻塞在 chan 的协程全部返回
	// 2. 读 closed 的 chan 会直接返回零值
	// 2. 写 closed 的 chan 会 panic，注意即使采用 select 非阻塞写 close 的 chan 也会 panic
	closeCh chan struct{}
}

func newClientConn(conn net.Conn, server *server) *clientConn {
	con := &clientConn{
		server:  server,
		conn:    conn,
		writeCh: make(chan *ResponseBody, writeChanSize),
		closeCh: make(chan struct{}),
	}
	// 大接口断言转换成小接口
	con.codec = server.codec(conn).(ServerCodec)

	go con.readLoop()
	go con.writeLoop()

	return con
}

func (conn *clientConn) readLoop() {
	defer conn.close()

	var err error
	var req *RequestBody

	for {
		req = new(RequestBody)
		err = conn.codec.ReadRequestBody(req)
		if err != nil {
			return
		}

		// 开启协程来处理请求
		// 不知道会产生什么错误？可能导致协程池失效？不理了。
		_ = gPool.Submit(conn.handleRequest(req))
	}
}

func (conn *clientConn) writeLoop() {
	defer conn.close()

	var err error
	for {
		select {
		case resp := <-conn.writeCh:
			select {
			case <-conn.closeCh:
				return
			default:
			}

			// 写入 socket
			err = conn.codec.WriteResponse(resp)
			if err != nil {
				return
			}

			// 拥有较高优先级
		case <-conn.closeCh:
			return
		}
	}
}

// handleRequest 处理每一个请求
func (conn *clientConn) handleRequest(req *RequestBody) func() {
	// 利用闭包来捕获变量
	return func() {
		timeout := time.Now().Add(sendTimeOutMicrosecond * time.Millisecond)

		// 构造 response
		resp := &ResponseBody{
			seq: req.seq,
		}

		byteView, err := conn.server.getValueFunc(req.key)

		// 为什么不像 transport.GetFromPeer 那种开启一个协程和一个计时器来实现超时？
		// 其实那种是超时了需要立刻返回的情况，但我这里超时了就超时了，不用一到超时时间就返回，可以一直等到超时结束
		if timeout.Before(time.Now()) {
			// 已经超时了，没有必要发过去了
			return
		}

		// 把 error 传递回给客户端
		if err == nil {
			resp.value = byteView.ByteSlice()
		} else {
			resp.err = err.Error()
		}

		// 发送给客户端
		select {
		case conn.writeCh <- resp:
		default:
		}
	}
}

func (conn *clientConn) close() {
	conn.closeDo.Do(func() {
		close(conn.closeCh)
		_ = conn.conn.Close()
	})
}
