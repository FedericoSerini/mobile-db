package crdt

import "github.com/federicoserini/mobile-db/core"

// Merge applies LWW (last-write-wins by HLC total order) over ops.
// Returns the winning value per doc to field. Safe to call with duplicates.
func Merge(ops []core.CRDTOp) map[string]map[string]any {
	type winner struct{ op core.CRDTOp }
	wins := map[string]map[string]winner{}

	for _, op := range ops {
		if _, ok := wins[op.DocID]; !ok {
			wins[op.DocID] = map[string]winner{}
		}
		cur, exists := wins[op.DocID][op.Field]
		if !exists || Before(cur.op.Timestamp, op.Timestamp) {
			wins[op.DocID][op.Field] = winner{op}
		}
	}

	result := make(map[string]map[string]any, len(wins))
	for docID, fields := range wins {
		result[docID] = make(map[string]any, len(fields))
		for field, w := range fields {
			result[docID][field] = w.op.Value
		}
	}
	return result
}
