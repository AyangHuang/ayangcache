package transport

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

type JsonCodec struct {
	conn io.ReadWriter
	// 为什么不用 readBuf，因为 json.Decoder 内部有一个 buf，类似与 bufio
	writeBuf *bufio.Writer
	dec      *json.Decoder
	enc      *json.Encoder
}

func NewJsonCodec(conn io.ReadWriter) Codec {
	// 对 conn 封装了 write buf，先写入 buf，满了就自动写入 conn，或调用 buf.Flush 主动将 buf 写入 conn
	buf := bufio.NewWriter(conn)
	return &JsonCodec{
		conn:     conn,
		writeBuf: buf,
		// 从 conn 的 socket 中读取到 json.Decoder.buf（相关于 bufio），再进行解码
		dec: json.NewDecoder(conn),
		// 编码后写入 bufio，再调用 bufio.flush 写入 conn 的 socket
		enc: json.NewEncoder(buf),
	}
}

func (codec *JsonCodec) ReadRequestBody(body *RequestBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}

	return codec.dec.Decode(body)
}
func (codec *JsonCodec) WriteResponse(body *ResponseBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}

	var err error
	err = codec.enc.Encode(body)
	err = codec.writeBuf.Flush()

	return err
}

func (codec *JsonCodec) ReadResponseBody(body *ResponseBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}
	return codec.dec.Decode(body)
}
func (codec *JsonCodec) WriteRequest(body *RequestBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}

	var err error
	err = codec.enc.Encode(body)
	err = codec.writeBuf.Flush()

	return err
}
