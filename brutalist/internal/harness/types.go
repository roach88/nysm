package harness

// Event is a marker interface for trace events.
// Both ir.Invocation and ir.Completion implement this interface.
type Event interface {
	// eventMarker is a private method to restrict implementers
	eventMarker()
}

// TraceEvent wraps either an invocation or completion for the trace.
// This provides a concrete type for the trace slice.
type TraceEvent struct {
	Type       string      `json:"type"` // "invocation" or "completion"
	ActionURI  string      `json:"action_uri,omitempty"`
	Args       interface{} `json:"args,omitempty"`
	OutputCase string      `json:"output_case,omitempty"`
	Result     interface{} `json:"result,omitempty"`
	Seq        int64       `json:"seq"`
}

// Result is the outcome of a test scenario execution.
type Result struct {
	// Pass indicates overall test success.
	// True if all expect clauses match.
	Pass bool `json:"pass"`

	// Trace contains all invocations and completions in order.
	// Used for trace assertions (Story 6.3) and golden comparison (Story 6.6).
	Trace []TraceEvent `json:"trace"`

	// Errors contains validation error messages.
	// Empty if Pass is true.
	Errors []string `json:"errors,omitempty"`

	// State contains final state tables for state assertions.
	// Keys are table names, values are query results.
	State map[string]interface{} `json:"state,omitempty"`
}

// NewResult creates a new passing result.
// Used as the starting point for test execution.
func NewResult() *Result {
	return &Result{
		Pass:   true,
		Trace:  []TraceEvent{},
		Errors: []string{},
		State:  make(map[string]interface{}),
	}
}

// AddError adds a validation error and marks the result as failed.
func (r *Result) AddError(err string) {
	r.Errors = append(r.Errors, err)
	r.Pass = false
}

// AddInvocationTrace adds an invocation to the trace.
func (r *Result) AddInvocationTrace(actionURI string, args interface{}, seq int64) {
	r.Trace = append(r.Trace, TraceEvent{
		Type:      "invocation",
		ActionURI: actionURI,
		Args:      args,
		Seq:       seq,
	})
}

// AddCompletionTrace adds a completion to the trace.
func (r *Result) AddCompletionTrace(outputCase string, result interface{}, seq int64) {
	r.Trace = append(r.Trace, TraceEvent{
		Type:       "completion",
		OutputCase: outputCase,
		Result:     result,
		Seq:        seq,
	})
}
