package transport

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// Server 这个 Server 是同步接受和发送的，串行化，处理完一个才能处理下一个
func Server(addr string) {
	m := make(map[string][]byte)
	m["ayang"] = []byte("ayang_value")
	m["tom"] = []byte("tom_value")
	m["timeout"] = []byte("timeout")
	m["close"] = []byte("close")

	var err error
	// 开启服务器
	server, err := net.Listen("tcp", addr)
	defer func() {
		_ = server.Close()
	}()

	if err != nil {
		fmt.Println(err.Error())
	}

	// 因为就一个客户端，就和客户端建立一条连接，所以不用 for
	conn, err := server.Accept()
	if err != nil {
		fmt.Println(err.Error())
	}
	codec := NewProtobufCodec(conn).(ServerCodec)
	for {
		var err error

		req := &RequestBody{}
		err = codec.ReadRequestBody(req)
		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println("-----server receive:", req.Key, ":", req.Seq)
		if req.Key == "timeout" {
			time.Sleep(7 * time.Second)
		} else if req.Key == "close" {
			// 关闭改条连接
			_ = conn.Close()
			// 关闭服务器监听 socket
			_ = server.Close()
			return
		}

		v, ok := m[req.Key]
		resp := &ResponseBody{
			Seq:   req.Seq,
			Value: v,
		}
		if !ok {
			resp.Err = "no this cache"
		}

		fmt.Println("server send", resp.Seq, ":", string(resp.Value))

		err = codec.WriteResponse(resp)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

}

func getFromPeer(t *transport, addr, key string, wg *sync.WaitGroup) {
	fmt.Println("----------client send")
	resp, err := t.GetFromPeer(addr, key)
	if err != nil {
		fmt.Println("-----------------------------client receive error", err.Error())
	} else {
		fmt.Println("----------client receive value:", string(resp))
	}

	wg.Done()
}

// 验证两种情况和另外一个功能：
// 1. 正常发送和接受
// 2. 服务器在处理发送错误并把错误发过来
// 3. 多个并发请求只会尝试建立连接一次
func TestTransport_GetFromPeer(t *testing.T) {
	serverAddr := "127.0.0.1:9999"
	// 简单服务器模拟，这个 Server 是同步接受和发送的，串行化，处理完一个才能处理下一个
	go Server(serverAddr)
	time.Sleep(time.Second)

	codec, _ := codecMap[ProtobufType]
	ts := &transport{
		client: newClient(codec),
	}

	wg := &sync.WaitGroup{}
	wg.Add(4)

	// 结果：立刻返回
	go getFromPeer(ts, serverAddr, "ayang", wg)
	go getFromPeer(ts, serverAddr, "tom", wg)
	go getFromPeer(ts, serverAddr, "ayang", wg)
	go getFromPeer(ts, serverAddr, "nocache", wg)
	wg.Wait()
	time.Sleep(time.Second)
}

// 验证服务器超时场景
func TestTransport_GetFromPeer_Timeout(t *testing.T) {
	serverAddr := "127.0.0.1:9999"
	go Server(serverAddr)
	time.Sleep(time.Second)

	codec, _ := codecMap[ProtobufType]
	ts := &transport{
		client: newClient(codec),
	}

	wg := &sync.WaitGroup{}
	wg.Add(3)

	// 结果：1 个成功，2 个 timeout
	go getFromPeer(ts, serverAddr, "timeout", wg)
	go getFromPeer(ts, serverAddr, "timeout", wg)
	go getFromPeer(ts, serverAddr, "timeout", wg)
	wg.Wait()
	time.Sleep(time.Second)
}

// 验证两种情况：
// 1. 服务器处理一半直接该 TCP 关闭（只是关闭该连接，当然会同时关闭监听 socket，用来验证下面的问题）
// 2. 发送时，发现服务器关闭。
func TestTransport_GetFromPeer_Close(t *testing.T) {
	serverAddr := "127.0.0.1:9999"
	go Server(serverAddr)

	time.Sleep(time.Second)

	codec, _ := codecMap[ProtobufType]
	ts := &transport{
		client: newClient(codec),
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	// 第一个服务器读取后，关闭该连接，然后关闭服务器监听 socket
	// 结果：两个都立刻返回
	getFromPeer(ts, serverAddr, "close", wg)
	getFromPeer(ts, serverAddr, "ayang", wg)
	wg.Wait()
	time.Sleep(time.Second)
}
