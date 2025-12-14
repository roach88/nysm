package ir

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONFieldNaming(t *testing.T) {
	inv := Invocation{
		FlowToken: "test-token",
		ActionURI: "Test.action",
		Seq:       42,
		SecurityContext: SecurityContext{
			TenantID: "t1",
			UserID:   "u1",
		},
	}
	data, err := json.Marshal(inv)
	require.NoError(t, err)

	// Verify snake_case JSON tags
	assert.Contains(t, string(data), `"flow_token"`)
	assert.Contains(t, string(data), `"action_uri"`)
	assert.Contains(t, string(data), `"security_context"`)
	assert.Contains(t, string(data), `"tenant_id"`)
	assert.Contains(t, string(data), `"user_id"`)

	// Verify NOT camelCase
	assert.NotContains(t, string(data), `"flowToken"`)
	assert.NotContains(t, string(data), `"actionUri"`)
	assert.NotContains(t, string(data), `"securityContext"`)
}

func TestEmptyStructMarshaling(t *testing.T) {
	tests := []struct {
		name string
		val  any
	}{
		{"ConceptSpec", ConceptSpec{}},
		{"ActionSig", ActionSig{}},
		{"SyncRule", SyncRule{}},
		{"Invocation", Invocation{}},
		{"Completion", Completion{}},
		{"SecurityContext", SecurityContext{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := json.Marshal(tt.val)
			require.NoError(t, err, "empty %s should marshal without panic", tt.name)
		})
	}
}

func TestSecurityContextAlwaysPresent(t *testing.T) {
	tests := []struct {
		name string
		inv  Invocation
	}{
		{"empty", Invocation{}},
		{"with_context", Invocation{SecurityContext: SecurityContext{UserID: "u1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.inv)
			require.NoError(t, err)
			assert.Contains(t, string(data), `"security_context"`)
		})
	}
}

func TestCompletionSecurityContextPresent(t *testing.T) {
	tests := []struct {
		name string
		comp Completion
	}{
		{"empty", Completion{}},
		{"with_context", Completion{SecurityContext: SecurityContext{UserID: "u1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.comp)
			require.NoError(t, err)
			assert.Contains(t, string(data), `"security_context"`)
		})
	}
}

func TestInvocationRoundTrip(t *testing.T) {
	inv := Invocation{
		ID:        "hash123",
		FlowToken: "flow-abc",
		ActionURI: "Order.place",
		Args:      IRObject{"item": IRString("widget"), "qty": IRInt(5)},
		Seq:       100,
		SecurityContext: SecurityContext{
			TenantID:    "tenant-1",
			UserID:      "user-1",
			Permissions: []string{"order:write"},
		},
		SpecHash:      "spec-hash",
		EngineVersion: EngineVersion,
		IRVersion:     IRVersion,
	}

	data, err := json.Marshal(inv)
	require.NoError(t, err)

	var decoded Invocation
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, inv.ID, decoded.ID)
	assert.Equal(t, inv.FlowToken, decoded.FlowToken)
	assert.Equal(t, inv.ActionURI, decoded.ActionURI)
	assert.Equal(t, inv.Seq, decoded.Seq)
	assert.Equal(t, inv.SecurityContext.TenantID, decoded.SecurityContext.TenantID)
	assert.Equal(t, inv.SecurityContext.UserID, decoded.SecurityContext.UserID)

	// Deep equality check for Args (AI-Review fix)
	require.Len(t, decoded.Args, 2)
	assert.Equal(t, IRString("widget"), decoded.Args["item"])
	assert.Equal(t, IRInt(5), decoded.Args["qty"])
}

func TestCompletionRoundTrip(t *testing.T) {
	comp := Completion{
		ID:           "comp-hash",
		InvocationID: "inv-hash",
		OutputCase:   "Success",
		Result:       IRObject{"order_id": IRString("ord-123")},
		Seq:          101,
		SecurityContext: SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}

	data, err := json.Marshal(comp)
	require.NoError(t, err)

	var decoded Completion
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, comp.ID, decoded.ID)
	assert.Equal(t, comp.InvocationID, decoded.InvocationID)
	assert.Equal(t, comp.OutputCase, decoded.OutputCase)

	// Deep equality check for Result (AI-Review fix)
	require.Len(t, decoded.Result, 1)
	assert.Equal(t, IRString("ord-123"), decoded.Result["order_id"])
}

func TestConceptSpecMarshaling(t *testing.T) {
	spec := ConceptSpec{
		Name:    "Order",
		Purpose: "Manage customer orders",
		StateSchema: []StateSchema{
			{
				Name:   "orders",
				Fields: map[string]string{"id": "string", "status": "string"},
			},
		},
		Actions: []ActionSig{
			{
				Name: "place",
				Args: []NamedArg{{Name: "item", Type: "string"}},
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{"order_id": "string"}},
					{Case: "OutOfStock", Fields: map[string]string{"item": "string"}},
				},
				Requires: []string{"order:write"},
			},
		},
		OperationalPrinciples: []OperationalPrinciple{
			{Description: "Orders can only be placed for available items", Scenario: "order-availability.yaml"},
		},
	}

	data, err := json.Marshal(spec)
	require.NoError(t, err)

	// Verify snake_case
	assert.Contains(t, string(data), `"state_schema"`)
	assert.Contains(t, string(data), `"operational_principles"`)

	var decoded ConceptSpec
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, spec.Name, decoded.Name)
	assert.Len(t, decoded.Actions, 1)
	assert.Equal(t, []string{"order:write"}, decoded.Actions[0].Requires)
}

func TestSyncRuleMarshaling(t *testing.T) {
	rule := SyncRule{
		ID:    "sync-1",
		Scope: ScopeSpec{Mode: "flow"},
		When: WhenClause{
			ActionRef:  "Order.place",
			EventType:  "completed",
			OutputCase: "Success",
			Bindings:   map[string]string{"order_id": "orderId"},
		},
		Where: &WhereClause{
			Source:   "inventory",
			Filter:   "item = bound.item",
			Bindings: map[string]string{"stock": "currentStock"},
		},
		Then: ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"order_id": "bound.orderId"},
		},
	}

	data, err := json.Marshal(rule)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"output_case"`)

	var decoded SyncRule
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, rule.ID, decoded.ID)
	assert.Equal(t, "Success", decoded.When.OutputCase)
}

func TestStoreTypesMarshaling(t *testing.T) {
	firing := SyncFiring{
		ID:           1,
		CompletionID: "comp-123",
		SyncID:       "sync-1",
		BindingHash:  "binding-hash",
		Seq:          50,
	}

	data, err := json.Marshal(firing)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"completion_id"`)
	assert.Contains(t, string(data), `"sync_id"`)
	assert.Contains(t, string(data), `"binding_hash"`)

	edge := ProvenanceEdge{
		ID:           1,
		SyncFiringID: 1,
		InvocationID: "inv-456",
	}

	data, err = json.Marshal(edge)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"sync_firing_id"`)
	assert.Contains(t, string(data), `"invocation_id"`)
}
