package zstd_test

import (
	"bytes"
	"testing"

	"github.com/federicoserini/mobile-db/compression/zstd"
)

func TestZstdRoundTrip(t *testing.T) {
	c := zstd.NewCompressor()
	original := []byte(`{"key":"value","number":42,"repeat":"aaaaaaaaaaaaaaaaaaaaaaaa"}`)
	compressed, err := c.Compress(original)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(decompressed, original) {
		t.Fatalf("mismatch: want %s, got %s", original, decompressed)
	}
}
