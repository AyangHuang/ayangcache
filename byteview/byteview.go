package byteview

// ByteView 缓存 value
type ByteView struct {
	b []byte
}

func (v ByteView) Len() int {
	return len(v.b)
}

func NewByteView(b []byte) ByteView {
	return ByteView{
		b: b,
	}
}

// ByteSlice 返回的是复制的值，防止缓存被更改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func StringToByteView(str string) ByteView {
	return ByteView{
		b: []byte(str),
	}
}

func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
