package transport

import (
	"ayangcache/byteview"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func init() {
	codec = codecMap[ProtobufType]
	cache = make(map[string]interface{})
	cache["ayang"] = []byte("ayangValue")
	cache["tom"] = []byte("tomValue")
}

var cache map[string]interface{}

var codec NewCodecFunc

var mockGetValueFunc GetValueFunc = func(key string) (byteview.ByteView, error) {
	v, ok := cache[key]
	if !ok {
		return byteview.ByteView{}, errors.New("have no this cache")
	}
	return byteview.NewByteView(v.([]byte)), nil
}

// 简单的串行化的 Client，反复进行读 request，写 response。不能边读边写。
func Client(keys ...string) {
	var err error
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	defer func() {
		_ = conn.Close()
	}()

	if err != nil {
		fmt.Println(err.Error())
	}

	codec := NewProtobufCodec(conn).(ClientCodec)

	var req *RequestBody
	var resp *ResponseBody

	for index := range keys {
		req = &RequestBody{
			seq: uint64(index),
			key: keys[index],
		}

		err = codec.WriteRequest(req)
		fmt.Println("------------client send", "seq:", req.seq, "key:", req.key)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		resp = &ResponseBody{}
		err = codec.ReadResponseBody(resp)

		fmt.Println("------------client receive", "seq:", resp.seq, "value:", string(resp.value), "err:", resp.err)
	}

}

func TestServer_Serve(t *testing.T) {
	server := newServer("127.0.0.1:9999", codec, mockGetValueFunc)
	go server.Serve()
	time.Sleep(time.Second)

	keys := []string{"ayang", "nocache", "tom"}

	Client(keys...)
	time.Sleep(time.Second)
}
