package transport

type RequestBody struct {
	Seq uint64 `json:"seq"`
	Key string `json:"key"`
}

type ResponseBody struct {
	Seq   uint64 `json:"seq"`
	Value []byte `json:"value"`
	Err   string `json:"err"`
}
