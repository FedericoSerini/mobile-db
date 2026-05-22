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
	enc, _ := gozstd.NewWriter(nil)
	dec, _ := gozstd.NewReader(nil)
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
