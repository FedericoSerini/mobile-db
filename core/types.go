package core

// HLC is a Hybrid Logical Clock timestamp providing total order across devices.
type HLC struct {
	WallTime int64  `cbor:"w"` // Unix nanoseconds
	Logical  uint32 `cbor:"l"` // monotonic counter
	DeviceID string `cbor:"d"` // tie-breaker
}

// CRDTOp is a single field mutation — commutative and idempotent.
type CRDTOp struct {
	OpID      string `cbor:"op_id"`
	DocID     string `cbor:"doc_id"`
	Field     string `cbor:"field"` // dot-separated JSON path
	Value     any    `cbor:"value"`
	Timestamp HLC    `cbor:"ts"`
	DeviceID  string `cbor:"device_id"`
}

// SyncMessage is sent client to server.
type SyncMessage struct {
	AppID               string   `cbor:"app_id"`
	UserID              string   `cbor:"user_id"`
	DatasetID           string   `cbor:"dataset_id"`
	Clock               HLC      `cbor:"clock"` // last known server HLC
	Ops                 []CRDTOp `cbor:"ops"`
	ClientSchemaVersion int      `cbor:"client_schema_version"`
}

// SyncResponse is sent server to client.
type SyncResponse struct {
	NewClock      HLC      `cbor:"new_clock"`
	Ops           []CRDTOp `cbor:"ops"`
	Snapshot      any      `cbor:"snapshot,omitempty"` // non-nil on schema version mismatch
	SchemaVersion int      `cbor:"schema_version"`
}

// DatasetACL controls read/write access to a dataset.
type DatasetACL string

const (
	ACLPrivate DatasetACL = "private"
	ACLShared  DatasetACL = "shared"
	ACLPublic  DatasetACL = "public"
)
