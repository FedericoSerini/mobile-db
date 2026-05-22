package core

import "context"

// StorageBackend is the database abstraction. All methods safe for concurrent use.
type StorageBackend interface {
	MergeOps(ctx context.Context, appID, userID, datasetID string, ops []CRDTOp) error
	GetDelta(ctx context.Context, appID, userID, datasetID string, since HLC) ([]CRDTOp, HLC, error)
	GetSnapshot(ctx context.Context, appID, userID, datasetID string) (map[string]any, HLC, error)
	SchemaVersion(ctx context.Context, appID, datasetID string) (int, error)
	OpCount(ctx context.Context, appID, userID, datasetID string) (int, error)
	Compact(ctx context.Context, appID, userID, datasetID string) error
	Close() error
}

// SyncTransport abstracts the network transport.
type SyncTransport interface {
	Receive(ctx context.Context) (<-chan SyncMessage, error)
	Send(ctx context.Context, clientID string, resp SyncResponse) error
	Notify(ctx context.Context, appID, userID string) error
}

// Codec serializes/deserializes sync payloads.
type Codec interface {
	Encode(v any) ([]byte, error)
	Decode(data []byte, v any) error
}

// Compressor compresses/decompresses serialized payloads.
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// No-op stubs for tests in other packages.

type NopStorage struct{}

func (NopStorage) MergeOps(_ context.Context, _, _, _ string, _ []CRDTOp) error { return nil }
func (NopStorage) GetDelta(_ context.Context, _, _, _ string, _ HLC) ([]CRDTOp, HLC, error) {
	return nil, HLC{}, nil
}
func (NopStorage) GetSnapshot(_ context.Context, _, _, _ string) (map[string]any, HLC, error) {
	return nil, HLC{}, nil
}
func (NopStorage) SchemaVersion(_ context.Context, _, _ string) (int, error) { return 1, nil }
func (NopStorage) OpCount(_ context.Context, _, _, _ string) (int, error)    { return 0, nil }
func (NopStorage) Compact(_ context.Context, _, _, _ string) error           { return nil }
func (NopStorage) Close() error                                               { return nil }

type NopTransport struct{}

func (NopTransport) Receive(_ context.Context) (<-chan SyncMessage, error) {
	ch := make(chan SyncMessage)
	close(ch)
	return ch, nil
}
func (NopTransport) Send(_ context.Context, _ string, _ SyncResponse) error { return nil }
func (NopTransport) Notify(_ context.Context, _, _ string) error             { return nil }

type NopCodec struct{}

func (NopCodec) Encode(_ any) ([]byte, error) { return nil, nil }
func (NopCodec) Decode(_ []byte, _ any) error { return nil }

type NopCompressor struct{}

func (NopCompressor) Compress(data []byte) ([]byte, error)   { return data, nil }
func (NopCompressor) Decompress(data []byte) ([]byte, error) { return data, nil }
