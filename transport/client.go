package transport

import (
	"ayangcache/singleflight"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

const (
	writeChanSize = 4096
)

// 起别名
type addr = string

type client struct {
	// 为什么读写锁？大部分时间都在读
	rw sync.RWMutex
	// map(server(ip:port), *peerConn)
	connMap map[addr]*peerConn
	// 在高并发下对于同一个服务端保证只创建一次连接
	singleCreateConn singleflight.Group
	// 编码格式
	codec NewCodecFunc
}

func newClient(codecFunc NewCodecFunc) *client {
	return &client{
		connMap:          make(map[string]*peerConn),
		singleCreateConn: singleflight.NewGroup(),
		codec:            codecFunc,
	}
}

func (c *client) send(call *call) {
	// 获取连接
	peerConn, err := c.getPeerConn(call.addr)

	if err != nil {
		call.err = err
		// 通知结束，必须非阻塞。因为如果超时了，已经没有接收方，发送会永久阻塞，造成协程泄露
		select {
		case call.valCh <- nil:
		default:
		}

		return
	}

	// 判断是否超时了，超时则 chan 已 close，会立刻返回，就不用发送了。否则走 default
	select {
	case <-call.timeout.Done():
		return
	default:
	}

	// 通过 TCP 连接发送
	peerConn.send(call)
}

// getPeerConn 获取连接，如没有则创建
func (c *client) getPeerConn(addr string) (*peerConn, error) {
	peerConn := c.getConnFromMap(addr)
	if peerConn != nil {
		// 已经有连接直接返回
		return peerConn, nil
	}

	// 没有则新创建一条 TCP 连接
	return c.createConn(addr)
}

func (c *client) getConnFromMap(addr string) *peerConn {
	c.rw.RLock()
	peerConn, ok := c.connMap[addr]
	c.rw.RUnlock()
	if ok {
		return peerConn
	}
	return nil
}

func (c *client) removeConn(addr string) {
	c.rw.Lock()
	delete(c.connMap, addr)
	log.Println("client removeConn", addr)
	c.rw.Unlock()
}

func (c *client) addConnToMap(addr string, conn *peerConn) {
	c.rw.Lock()
	c.connMap[addr] = conn
	c.rw.Unlock()
}

func (c *client) createConn(addr string) (*peerConn, error) {
	// 利用 singleFlight 进行一次创建连接
	p, err := c.singleCreateConn.Do(addr, func() (interface{}, error) {
		// 再检查一遍
		conn := c.getConnFromMap(addr)
		if conn != nil {
			return conn, nil
		}

		// 建立连接
		conn, err := newPeerConn(addr, c)
		if err != nil {
			return nil, err
		}

		// 加入客户端的连接池中
		c.addConnToMap(addr, conn)
		return conn, nil
	})

	if err != nil {
		return nil, err
	}

	return p.(*peerConn), nil
}

// 起别名
type seq = uint64

type peerConn struct {
	// 主要是想获取 client.codec，还有从 client 删除这个 peerConn
	client *client
	// 0 表示心跳请求
	nextSeq atomic.Uint64
	// 连接的服务端节点地址
	serverAddr string
	// 真正的 TCP 连接
	conn net.Conn
	// codec 编码方式
	codec     ClientCodec
	callMutex sync.Mutex
	// 记录所有发送过的请求，发送后根据 seq 找到 call，传入 value 唤醒阻塞的 G
	calls map[seq]*call
	// 与写协程通信
	// 注意：这个 chan 是不会永远不会被关闭的，因为需要不断写入。（GC 会回收 chan 根据的是能不能到达，而不是是否已经 closed）
	// 没有办法可以判断一个 chan 是否被关闭（if v, ok := <-chan; ok { }，这种不能算，且 ok 表示的是是否有数据可读）
	// 通过非阻塞写入来防止协程泄露
	writeCh chan *RequestBody
	// 保证只执行一次关闭
	closeDo sync.Once
	// 关闭信号，采用直接 close chan 的方式
	// close chan 会：
	// 1. 导致阻塞在 chan 的协程全部返回
	// 2. 读 closed 的 chan 会直接返回零值
	// 2. 写 closed 的 chan 会 panic，注意即使采用 select 非阻塞写 close 的 chan 也会 panic
	closeCh chan struct{}
}

func newPeerConn(serverAddr string, client *client) (*peerConn, error) {
	// 客户端对服务端发起建立 TCP 连接的请求
	log.Println("尝试与", serverAddr, "建立连接")
	conn, err := net.Dial("tcp", serverAddr)

	if err != nil {
		return nil, err
	}
	log.Println("与", serverAddr, "连接建立成功")

	pConn := &peerConn{
		client:     client,
		nextSeq:    atomic.Uint64{},
		serverAddr: serverAddr,
		conn:       conn,
		calls:      make(map[uint64]*call),
		writeCh:    make(chan *RequestBody, writeChanSize),
		closeCh:    make(chan struct{}),
	}
	// 加入编码方式。通过断言，大 interface.(小 interface)，大接口变成小接口。好处：不需要的方法就不显示了，防止错误调用
	pConn.codec = client.codec(pConn.conn).(ClientCodec)

	// 读写分离，开启两个协程进行读写
	go pConn.writeLoop()
	go pConn.readLoop()

	return pConn, nil
}

// writeLoop 一条 TCP 连接对于一个守护写协程
func (conn *peerConn) writeLoop() {
	defer conn.close()

	var err error
	var req *RequestBody

	for {
		select {
		case req = <-conn.writeCh:
			// 验证是否已经关闭
			select {
			case <-conn.closeCh:
				return
			default:
			}

			err = conn.codec.WriteRequest(req)
			if err != nil {
				// 这里为了简单，有任何错误（eg：EOF（对方 close TCP），我方序列化错误等），直接关闭连接
				log.Println("writeLoop error:", err.Error())
				return
			}

			// 优先级高，因为上面有验证
		case <-conn.closeCh:
			return
		}
	}
}

// readLoop 一条 TCP 连接对于一个守护读协程
func (conn *peerConn) readLoop() {
	defer conn.close()

	var err error
	var resp *ResponseBody

	for {
		resp = new(ResponseBody)
		// 获取一个 response
		err = conn.codec.ReadResponseBody(resp)

		// 也是遇到错误直接关闭
		if err != nil {
			log.Println("readLoop error:", err.Error())
			return
		}

		// 处理 response
		call := conn.searchCall(resp.Seq)
		if call == nil {
			// 说明已经被删除了，即超时了，所以不用处理
			continue
		}

		// 服务器发生的错误
		if resp.Err != "" {
			call.err = errors.New(resp.Err)
		}

		// 必须非阻塞发送，保证超时了就没有接收方在等待该 chan。导致发送阻塞，协程泄露
		// 注意：上面已经判断过超时了，为什么这里还会出现超时的情况：removeCall 和 超时返回不是在同一个协程，即不是原子的。
		// 具体看 transport.GetFromPeer 和 peerConn.send
		select {
		// 发送给阻塞等待方
		case call.valCh <- resp.Value:
		default:
		}
	}
}

func (conn *peerConn) send(c *call) {
	// 原子增加并返回
	c.Seq = conn.nextSeq.Add(1)

	// 注册 call
	// 什么时候应该把 call 从 map 中删除呢？
	// 1. 在读 response 后找到后删除
	// 2. 超时，两种情况（1）己方发不过去（2）对方没发过来
	conn.addCall(c)

	// 还是必须非阻塞，可能已经关闭该 conn（关闭该 conn 怎么走到这里？因为关闭前就已经获得 conn 了，走到这里的中间被关闭了 ）
	// 那要是
	select {
	case conn.writeCh <- c.RequestBody:
	default:
	}

	// 上面两种情况都在这里删除
	// 情况 1：正常或异常返回后也会 cancel，本质也是 close 这个 chan
	// 情况 2：超时会 close 这个 chan，直接返回
	// 下面的 chan 是等待上面这俩种情况返回
	// 可以看到 context.Timeout 是直接关闭这个 chan（这个chan是一次性的）。这样反而用处很大，可以多个 G 等待这个chan
	// 而 time.After 这种，时间到了发送一个 item 到 chan 里，只适合一个 G 在等待
	<-c.timeout.Done()
	conn.removeCall(c.Seq)
}

func (conn *peerConn) removeCall(seq uint64) {
	conn.callMutex.Lock()
	delete(conn.calls, seq)
	log.Println("peerConn removeCall", "seq:", seq)
	conn.callMutex.Unlock()
}

func (conn *peerConn) addCall(c *call) {
	conn.callMutex.Lock()
	conn.calls[c.Seq] = c
	log.Println("peerConn addCall", "seq:", c.Seq, "key:", c.Key)
	conn.callMutex.Unlock()
}

func (conn *peerConn) searchCall(seq uint64) *call {
	conn.callMutex.Lock()
	call, ok := conn.calls[seq]
	conn.callMutex.Unlock()

	if !ok {
		return nil
	}
	return call
}

func (conn *peerConn) clearCalls() {
	conn.callMutex.Lock()
	// 唤醒所有阻塞等待返回的请求协程
	for _, call := range conn.calls {
		call.err = errors.New("with " + conn.serverAddr + " connection has closed")
		select {
		case call.valCh <- nil:
		default:
		}
	}
	// 清空 map
	conn.calls = nil
	conn.callMutex.Unlock()
}

func (conn *peerConn) close() {
	conn.closeDo.Do(func() {
		// 移出 client map
		conn.client.removeConn(conn.serverAddr)

		// 唤醒所有阻塞等待返回的请求协程
		conn.clearCalls()

		// 发送信号通知
		close(conn.closeCh)
		// 关闭 TCP 连接
		_ = conn.conn.Close()
	})
}
