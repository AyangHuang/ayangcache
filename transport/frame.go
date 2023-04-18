package transport

type RequestBody struct {
	seq uint64
	key string
}

type ResponseBody struct {
	seq   uint64
	value []byte
	err   string
}
