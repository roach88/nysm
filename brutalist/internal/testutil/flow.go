package testutil

// FixedFlowGenerator generates the same flow token every time.
//
// This enables deterministic test execution and golden snapshot comparison.
// The same scenario with the same FixedFlowGenerator produces byte-identical event logs.
//
// Unlike engine.FixedGenerator which returns tokens in sequence, this generator
// always returns the same token. This is useful for scenarios where all events
// should share the same flow token.
//
// Thread-safety: FixedFlowGenerator is stateless and safe for concurrent use.
type FixedFlowGenerator struct {
	token string
}

// NewFixedFlowGenerator creates a new fixed flow token generator.
//
// The token is typically set in the scenario YAML:
//
//	flow_token: "test-flow-00000000-0000-0000-0000-000000000001"
//
// If token is empty, Generate() returns "test-flow-default".
func NewFixedFlowGenerator(token string) *FixedFlowGenerator {
	if token == "" {
		token = "test-flow-default"
	}
	return &FixedFlowGenerator{token: token}
}

// Generate returns the fixed flow token.
//
// Implements engine.FlowTokenGenerator interface.
func (g *FixedFlowGenerator) Generate() string {
	return g.token
}
