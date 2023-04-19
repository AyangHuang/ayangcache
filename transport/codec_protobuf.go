package transport

import (
	"ayangcache/transport/protobuf"
	"bufio"
	"errors"
	protoG "github.com/golang/protobuf/proto"
	"io"
)

const (
	// 采用 protobuf 编码的话，前 2 个字节为 body 长度，后面则为采用 protobuf 编码后的 body
	// 注意：如果采用 json 编码，传长度，因为 json 能自动识别出边界
	frameLength = 2
)

type ProtobufCodec struct {
	conn    io.ReadWriter
	readBuf *bufio.Reader
	// 对 conn 封装 write buf，先写入 buf，满了就自动写入 conn，或调用 buf.Flush 主动将 buf 写入 conn
	writeBuf *bufio.Writer
}

func NewProtobufCodec(conn io.ReadWriter) Codec {
	writeBuf := bufio.NewWriter(conn)
	readBuf := bufio.NewReader(conn)
	return &ProtobufCodec{
		conn:     conn,
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}
}

// readFrameLen 读 2 个 Byte 的长度
func (proto *ProtobufCodec) readFrameLen() (uint16, error) {
	lenBytes := make([]byte, frameLength)

	// 从 socket 中读
	length, err := proto.readBuf.Read(lenBytes)
	if err != nil {
		return 0, err
	}
	if length != frameLength {
		return 0, errors.New("read length error")
	}

	return bytesToUint16(lenBytes), nil
}

func (proto *ProtobufCodec) readBody(bytes []byte, lenBytes uint16) error {
	length, err := proto.readBuf.Read(bytes)
	if err != nil {
		return err
	}
	if length != int(lenBytes) {
		return errors.New("read body error")
	}
	return nil
}

func (proto *ProtobufCodec) ReadRequestBody(body *RequestBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}

	// 先读 2 个字节的长度
	var err error
	length, err := proto.readFrameLen()

	if err != nil {
		return err
	}

	// 按照长度读 body
	data := make([]byte, length)
	if err = proto.readBody(data, length); err != nil {
		return err
	}

	// 反序列化
	pBody := new(protobuf.RequestBody)
	err = protoG.Unmarshal(data, pBody)
	if err != nil {
		return err
	}

	body.Seq = pBody.GetSeq()
	body.Key = pBody.GetKey()

	return nil

}

func (proto *ProtobufCodec) ReadResponseBody(body *ResponseBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}

	// 先读 2 个字节的长度
	var err error
	length, err := proto.readFrameLen()

	if err != nil {
		return err
	}

	// 按照长度读 body
	data := make([]byte, length)
	if err = proto.readBody(data, length); err != nil {
		return err
	}

	// 反序列化
	pBody := new(protobuf.ResponseBody)
	err = protoG.Unmarshal(data, pBody)
	if err != nil {
		return err
	}

	body.Seq = pBody.GetSeq()
	body.Value = pBody.GetValue()
	body.Err = pBody.GetErr()
	return nil
}

func (proto *ProtobufCodec) WriteRequest(body *RequestBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}

	var err error
	message := &protobuf.RequestBody{
		Seq: body.Seq,
		Key: body.Key,
	}

	// 需要验证大小，超出 16 bit 不行，这里就不处理了
	bytes, err := protoG.Marshal(message)
	if err != nil {
		return err
	}

	lenBytes := uint16ToBytes(uint16(len(bytes)))

	_, err = proto.writeBuf.Write(lenBytes)
	_, err = proto.writeBuf.Write(bytes)

	// 刷新入 socket
	err = proto.writeBuf.Flush()
	return err
}

func (proto *ProtobufCodec) WriteResponse(body *ResponseBody) error {
	if body == nil {
		return errors.New("nil pointer")
	}

	var err error
	message := &protobuf.ResponseBody{
		Seq:   body.Seq,
		Value: body.Value,
		Err:   body.Err,
	}

	// 需要验证大小，超出 16 bit 不行。简单一点，这里就不处理了。
	bytes, err := protoG.Marshal(message)
	if err != nil {
		return err
	}

	lenBytes := uint16ToBytes(uint16(len(bytes)))

	_, err = proto.writeBuf.Write(lenBytes)
	_, err = proto.writeBuf.Write(bytes)

	// 刷新入 socket
	err = proto.writeBuf.Flush()
	return err
}

func bytesToUint16(bytes []byte) uint16 {
	// 第一个字节表示高位，第二个字节表示低位
	return uint16(bytes[0])<<8 | uint16(bytes[1])
}

func uint16ToBytes(num uint16) []byte {
	// 第一个字节表示高位，第二个字节表示低位
	return []byte{byte(num >> 8), byte(num & 0xff)}
}
