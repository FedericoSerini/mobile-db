package cbor

import (
	gocbor "github.com/fxamacker/cbor/v2"
)

type Codec struct {
	enc gocbor.EncMode
	dec gocbor.DecMode
}

func NewCodec() *Codec {
	enc, _ := gocbor.EncOptions{}.EncMode()
	dec, _ := gocbor.DecOptions{}.DecMode()
	return &Codec{enc: enc, dec: dec}
}

func (c *Codec) Encode(v any) ([]byte, error) {
	return c.enc.Marshal(v)
}

func (c *Codec) Decode(data []byte, v any) error {
	return c.dec.Unmarshal(data, v)
}
