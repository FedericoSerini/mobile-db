package zstd

import (
	"fmt"

	gozstd "github.com/klauspost/compress/zstd"
)

type Compressor struct {
	enc *gozstd.Encoder
	dec *gozstd.Decoder
}

func NewCompressor() *Compressor {
	enc, err := gozstd.NewWriter(nil)
	if err != nil {
		panic("zstd: failed to create encoder: " + err.Error())
	}
	dec, err := gozstd.NewReader(nil)
	if err != nil {
		panic("zstd: failed to create decoder: " + err.Error())
	}
	return &Compressor{enc: enc, dec: dec}
}

func (c *Compressor) Compress(data []byte) ([]byte, error) {
	return c.enc.EncodeAll(data, nil), nil
}

func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	out, err := c.dec.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decompress: %w", err)
	}
	return out, nil
}
