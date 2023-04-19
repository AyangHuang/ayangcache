package transport

import "testing"

func TestJsonCodec_WriteRequest_ReadRequestBody(t *testing.T) {
	stream := &stream{
		bytes: make([]byte, 4096, 4096),
	}
	c := NewJsonCodec(stream)

	req1 := &RequestBody{
		Seq: 1,
		Key: "ayang",
	}
	req2 := &RequestBody{
		Seq: 2,
		Key: "tom",
	}

	_ = c.WriteRequest(req1)
	_ = c.WriteRequest(req2)

	var req11 = new(RequestBody)
	var req22 = new(RequestBody)

	_ = c.ReadRequestBody(req11)
	_ = c.ReadRequestBody(req22)

	if req1.Seq == req11.Seq && req1.Key == req11.Key {
	} else {
		t.Error("error")
	}
	if req2.Seq == req22.Seq && req2.Key == req22.Key {
	} else {
		t.Error("error")
	}
}

func TestJsonCodec_WriteResponse_ReadResponseBody(t *testing.T) {
	stream := &stream{
		bytes: make([]byte, 4096, 4096),
	}
	c := NewJsonCodec(stream)

	req1 := &ResponseBody{
		Seq:   1,
		Value: []byte("ayang"),
	}
	req2 := &ResponseBody{
		Seq:   2,
		Value: []byte("tom"),
	}

	_ = c.WriteResponse(req1)
	_ = c.WriteResponse(req2)

	var req11 = new(ResponseBody)
	var req22 = new(ResponseBody)

	_ = c.ReadResponseBody(req11)
	_ = c.ReadResponseBody(req22)

	if req1.Seq == req11.Seq && string(req1.Value) == string(req11.Value) {
	} else {
		t.Error("error")
	}
	if req2.Seq == req22.Seq && string(req2.Value) == string(req22.Value) {
	} else {
		t.Error("error")
	}
}
