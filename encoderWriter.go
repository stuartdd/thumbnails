package main

type EncodedWriter struct {
	bytes []byte
	pos   int
	ext   int
	size  int
}

func NewEncodedWriter(ext int) *EncodedWriter {
	if ext < 2 {
		panic("Encoded Writer extension must be more than 1")
	}
	b := make([]byte, ext)
	return &EncodedWriter{bytes: b, pos: 0, ext: ext, size: len(b)}
}

func (ew *EncodedWriter) Bytes() []byte {
	return ew.bytes[0:ew.pos]
}

func (ew *EncodedWriter) Write(p []byte) (n int, err error) {
	pos := ew.pos
	for _, b := range p {
		if pos >= ew.size {
			ew.bytes = append(ew.bytes, make([]byte, ew.ext)...)
			ew.size = len(ew.bytes)
		}
		ew.bytes[pos] = b
		pos++
	}
	ew.pos = pos
	return len(p), nil
}
