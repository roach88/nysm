package ir

// NOTE: These are store-internal types, not part of the canonical IR.
// They use auto-increment IDs for FK references (exception to CP-2).

// SyncFiring represents a sync rule firing record (store-layer).
type SyncFiring struct {
	ID           int64  `json:"id"`            // Auto-increment (store FK)
	CompletionID string `json:"completion_id"` // Content-addressed
	SyncID       string `json:"sync_id"`
	BindingHash  string `json:"binding_hash"` // Hash of binding values (CP-1)
	Seq          int64  `json:"seq"`          // Logical clock
}

// ProvenanceEdge links a sync firing to its generated invocation (store-layer).
type ProvenanceEdge struct {
	ID           int64  `json:"id"`             // Auto-increment (store FK)
	SyncFiringID int64  `json:"sync_firing_id"`
	InvocationID string `json:"invocation_id"` // Content-addressed
}
