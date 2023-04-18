package transport

import (
	"io"
)

type NewCodecFunc func(io.ReadWriter) Codec

const (
	ProtobufType = "protobuf"
	JsonType     = "json"
)

var codecMap = make(map[string]NewCodecFunc)

func init() {
	codecMap[ProtobufType] = NewProtobufCodec
}

type Codec interface {
	ServerCodec
	ClientCodec
}

type ServerCodec interface {
	ReadRequestBody(body *RequestBody) error
	WriteResponse(body *ResponseBody) error
}

type ClientCodec interface {
	ReadResponseBody(body *ResponseBody) error
	WriteRequest(body *RequestBody) error
}
