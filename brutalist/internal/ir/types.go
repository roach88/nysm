package ir

// ConceptSpec represents a compiled concept definition.
type ConceptSpec struct {
	Name                  string                 `json:"name"`
	Purpose               string                 `json:"purpose"`
	StateSchema           []StateSchema          `json:"state_schema"`
	Actions               []ActionSig            `json:"actions"`
	OperationalPrinciples []OperationalPrinciple `json:"operational_principles"`
}

// ActionSig represents an action signature with typed inputs/outputs.
type ActionSig struct {
	Name     string       `json:"name"`
	Args     []NamedArg   `json:"args"`
	Outputs  []OutputCase `json:"outputs"`
	Requires []string     `json:"requires,omitempty"` // Required permissions (authz)
}

// OutputCase represents a typed output variant (success or error).
type OutputCase struct {
	Case   string            `json:"case"`   // "Success", "InsufficientStock", etc.
	Fields map[string]string `json:"fields"` // field name -> type name
}

// StateSchema represents a state table definition.
type StateSchema struct {
	Name   string            `json:"name"`
	Fields map[string]string `json:"fields"` // field name -> type name
}

// NamedArg represents a named argument with type.
type NamedArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// OperationalPrinciple represents a testable behavioral contract.
type OperationalPrinciple struct {
	Description string `json:"description"`
	Scenario    string `json:"scenario"` // Path to scenario file or inline
}

// SyncRule represents a compiled sync rule (when/where/then).
type SyncRule struct {
	ID    string       `json:"id"`
	Scope ScopeSpec    `json:"scope"`
	When  WhenClause   `json:"when"`
	Where *WhereClause `json:"where,omitempty"` // Optional
	Then  ThenClause   `json:"then"`
}

// ScopeSpec defines the scoping mode for a sync rule.
type ScopeSpec struct {
	Mode string `json:"mode"`          // "flow", "global", or "keyed"
	Key  string `json:"key,omitempty"` // field name for keyed mode
}

// ValidScopeModes defines allowed scope modes.
var ValidScopeModes = map[string]bool{
	"flow":   true,
	"global": true,
	"keyed":  true,
}

// Invocation represents an action invocation record.
type Invocation struct {
	ID              string          `json:"id"`               // Content-addressed hash
	FlowToken       string          `json:"flow_token"`
	ActionURI       ActionRef       `json:"action_uri"`       // Typed action reference
	Args            IRObject        `json:"args"`             // Constrained to IRValue types
	Seq             int64           `json:"seq"`              // Logical clock (CP-2)
	SecurityContext SecurityContext `json:"security_context"` // Always present (CP-6)
	SpecHash        string          `json:"spec_hash"`        // Hash of concept spec
	EngineVersion   string          `json:"engine_version"`   // Engine version
	IRVersion       string          `json:"ir_version"`       // IR schema version
}

// Completion represents an action completion record.
type Completion struct {
	ID              string          `json:"id"`               // Content-addressed hash
	InvocationID    string          `json:"invocation_id"`
	OutputCase      string          `json:"output_case"`      // "Success", error variant
	Result          IRObject        `json:"result"`           // Constrained to IRValue types
	Seq             int64           `json:"seq"`              // Logical clock (CP-2)
	SecurityContext SecurityContext `json:"security_context"` // Always present (CP-6)
}

// SecurityContext contains security metadata for audit trails (CP-6).
// MUST be non-pointer and always present on Invocation and Completion.
type SecurityContext struct {
	TenantID    string   `json:"tenant_id"`
	UserID      string   `json:"user_id"`
	Permissions []string `json:"permissions"`
}
