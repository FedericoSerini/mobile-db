package core_test

import "github.com/federicoserini/mobile-db/core"

// Compile-time interface compliance checks.
var _ core.StorageBackend = (*core.NopStorage)(nil)
var _ core.SyncTransport = (*core.NopTransport)(nil)
var _ core.Codec = (*core.NopCodec)(nil)
var _ core.Compressor = (*core.NopCompressor)(nil)
