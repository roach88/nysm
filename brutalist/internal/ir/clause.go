package ir

// WhenClause specifies what completion triggers the sync.
type WhenClause struct {
	ActionRef  string            `json:"action_ref"`            // "Cart.checkout"
	EventType  string            `json:"event_type"`            // "completed" or "invoked"
	OutputCase string            `json:"output_case,omitempty"` // "Success", etc. (empty = match any)
	Bindings   map[string]string `json:"bindings"`              // var name → path expression
}

// WhereClause specifies the query to produce bindings.
type WhereClause struct {
	Source   string            `json:"source"`   // State reference e.g. "CartItem"
	Filter   string            `json:"filter"`   // Filter expression e.g. "cart_id == bound.cart_id"
	Bindings map[string]string `json:"bindings"` // var name → path expression
}

// ThenClause specifies the action to invoke.
type ThenClause struct {
	ActionRef string            `json:"action_ref"` // "Inventory.reserve"
	Args      map[string]string `json:"args"`       // arg name → expression using bound vars
}
