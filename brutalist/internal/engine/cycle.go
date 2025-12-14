package engine

import "sync"

// CycleDetector tracks sync firings per flow to prevent infinite loops.
//
// Cycles occur when the same (sync_id, binding_hash) would fire multiple
// times in a single flow. This happens with self-referential or mutually
// recursive sync rules.
//
// Example cycle:
//
//	Order.Create completes → sync-reserve fires → Inventory.ReserveStock completes
//	→ sync-create-order fires → Order.Create completes (again!)
//	→ sync-reserve would fire again... ← CYCLE DETECTED
//
// The detector maintains per-flow history of (sync_id, binding_hash) pairs
// that have already fired. Before each firing, WouldCycle() checks if the
// pair has been seen before in this flow.
//
// CRITICAL DISTINCTION from Idempotency (Story 5.1):
//   - Idempotency: "Have we fired this (completion, sync, binding) triple?" (persistent)
//   - Cycle Detection: "Have we fired this (sync, binding) in this flow?" (in-memory)
//
// Both checks are required:
//   - Idempotency prevents duplicate firings on crash/replay
//   - Cycle detection prevents infinite loops during execution
type CycleDetector struct {
	mu      sync.Mutex
	history map[string]map[string]bool // map[flow_token]map[cycle_key]bool
}

// NewCycleDetector creates a new cycle detector.
func NewCycleDetector() *CycleDetector {
	return &CycleDetector{
		history: make(map[string]map[string]bool),
	}
}

// WouldCycle checks if firing this (sync_id, binding_hash) would create a cycle.
//
// Returns true if the same (sync_id, binding_hash) has already fired in this flow.
// Returns false for the first occurrence or if flow has no history.
//
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) WouldCycle(flowToken, syncID, bindingHash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// No history for this flow = first time seeing it
	if c.history[flowToken] == nil {
		return false
	}

	cycleKey := syncID + ":" + bindingHash
	return c.history[flowToken][cycleKey]
}

// Record marks that this (sync_id, binding_hash) has fired in this flow.
//
// This should be called immediately after WouldCycle() returns false,
// before actually firing the sync rule.
//
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) Record(flowToken, syncID, bindingHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialize flow history if needed
	if c.history[flowToken] == nil {
		c.history[flowToken] = make(map[string]bool)
	}

	cycleKey := syncID + ":" + bindingHash
	c.history[flowToken][cycleKey] = true
}

// Clear removes all history for a flow token.
//
// Used when:
//   - Flow completes successfully (cleanup)
//   - Flow terminates with error (cleanup)
//   - Testing (reset state between tests)
//
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) Clear(flowToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.history, flowToken)
}

// HistorySize returns the number of flows with tracked history.
//
// Used for testing and introspection.
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) HistorySize() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.history)
}

// FlowHistorySize returns the number of (sync, binding) pairs tracked for a flow.
//
// Used for testing and introspection.
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) FlowHistorySize(flowToken string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.history[flowToken] == nil {
		return 0
	}
	return len(c.history[flowToken])
}
