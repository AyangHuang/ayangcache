package transport

import (
	"ayangcache/transport/protobuf"
	"fmt"
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestBytesToUint16_Uint16ToBytes(t *testing.T) {
	var i uint16 = 13145
	if bytesToUint16(uint16ToBytes(i)) != i {
		t.Error("accept 13145, but", i)
	}
}

type stream struct {
	bytes     []byte
	readNext  int
	writeNext int
}

func (s *stream) Write(p []byte) (n int, err error) {
	length := copy(s.bytes[s.writeNext:], p)
	s.writeNext += length
	if length != len(p) {
		panic("write error")
	}
	return length, nil
}

func (s *stream) Read(p []byte) (n int, err error) {
	length := copy(p, s.bytes[s.readNext:])
	s.readNext += length
	if length != len(p) {
		panic("read error")
	}
	return length, nil
}

func TestProtobufCodec_WriteRequest_ReadRequestBody(t *testing.T) {
	stream := &stream{
		bytes: make([]byte, 4096, 4096),
	}
	c := NewProtobufCodec(stream)

	req1 := &RequestBody{
		seq: 1,
		key: "ayang",
	}
	req2 := &RequestBody{
		seq: 2,
		key: "tom",
	}

	_ = c.WriteRequest(req1)
	_ = c.WriteRequest(req2)

	var req11 *RequestBody = new(RequestBody)
	var req22 *RequestBody = new(RequestBody)

	_ = c.ReadRequestBody(req11)
	_ = c.ReadRequestBody(req22)

	if req1.seq == req11.seq && req1.key == req11.key {
	} else {
		t.Error("error")
	}
	if req2.seq == req22.seq && req2.key == req22.key {
	} else {
		t.Error("error")
	}
}

func TestProtobufCodec_WriteResponse_ReadResponseBody(t *testing.T) {
	stream := &stream{
		bytes: make([]byte, 4096, 4096),
	}
	c := NewProtobufCodec(stream)

	req1 := &ResponseBody{
		seq:   1,
		value: []byte("ayang"),
	}
	req2 := &ResponseBody{
		seq:   2,
		value: []byte("tom"),
	}

	_ = c.WriteResponse(req1)
	_ = c.WriteResponse(req2)

	var req11 *ResponseBody = new(ResponseBody)
	var req22 *ResponseBody = new(ResponseBody)

	_ = c.ReadResponseBody(req11)
	_ = c.ReadResponseBody(req22)

	if req1.seq == req11.seq && string(req1.value) == string(req11.value) {
	} else {
		t.Error("error")
	}
	if req2.seq == req22.seq && string(req2.value) == string(req22.value) {
	} else {
		t.Error("error")
	}
}

// TestProtobuf 测试了：不能使用流的方式进行 protobuf 解码，必须一个完整的 []byte，不能多，也不能少。
// 其实 api 的注释有写了。（但是看不懂 wire-format 什么意思啊啊啊）
// Unmarshal parses a wire-format message in b
// func Unmarshal(b []byte, m Message) error
func TestProtobuf(t *testing.T) {
	resp := &ResponseBody{
		seq:   2,
		value: []byte("tom"),
	}

	var err error
	message1 := &protobuf.ResponseBody{
		Seq:   resp.seq,
		Value: resp.value,
	}
	message2 := &protobuf.ResponseBody{
		Seq:   resp.seq,
		Value: resp.value,
	}

	bytes, err := proto.Marshal(message1)
	bytes2, err := proto.Marshal(message2)

	bytess := make([]byte, len(bytes)+len(bytes2)+10)
	copy(bytess, bytes)
	copy(bytess[len(bytes):], bytes2)
	bytess[len(bytes)+len(bytes2)] = '1'
	bytess[len(bytes)+len(bytes2)] = '2'

	respMessage := new(protobuf.ResponseBody)
	err = proto.Unmarshal(bytess, respMessage)
	err = proto.Unmarshal(bytess[len(bytes):], respMessage)
	if err == nil {
		fmt.Println(respMessage.GetValue())
	} else {
		println(err.Error()) // proto: cannot parse invalid wire-format data
	}
}
