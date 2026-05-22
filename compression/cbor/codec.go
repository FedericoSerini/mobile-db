package cbor

import (
	gocbor "github.com/fxamacker/cbor/v2"
)

type Codec struct{}

func NewCodec() *Codec { return &Codec{} }

func (c *Codec) Encode(v any) ([]byte, error) {
	return gocbor.Marshal(v)
}

func (c *Codec) Decode(data []byte, v any) error {
	return gocbor.Unmarshal(data, v)
}
