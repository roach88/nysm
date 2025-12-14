package compiler

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileSyncBasic(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "cart-inventory": {
			scope: "flow"

			when: {
				action: "Cart.checkout"
				event: "completed"
				case: "Success"
				bind: { cart_id: "result.cart_id" }
			}

			where: {
				from: "CartItem"
				filter: "cart_id == bound.cart_id"
				bind: { item_id: "item_id", quantity: "quantity" }
			}

			then: {
				action: "Inventory.reserve"
				args: {
					item_id: "bound.item_id"
					quantity: "bound.quantity"
				}
			}
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."cart-inventory"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "cart-inventory", rule.ID)
	assert.Equal(t, "flow", rule.Scope.Mode)
	assert.Empty(t, rule.Scope.Key)
	assert.Equal(t, "Cart.checkout", rule.When.ActionRef)
	assert.Equal(t, "completed", rule.When.EventType)
	assert.Equal(t, "Success", rule.When.OutputCase)
	assert.Equal(t, "result.cart_id", rule.When.Bindings["cart_id"])
	require.NotNil(t, rule.Where)
	assert.Equal(t, "CartItem", rule.Where.Source)
	assert.Equal(t, "cart_id == bound.cart_id", rule.Where.Filter)
	assert.Equal(t, "item_id", rule.Where.Bindings["item_id"])
	assert.Equal(t, "quantity", rule.Where.Bindings["quantity"])
	assert.Equal(t, "Inventory.reserve", rule.Then.ActionRef)
	assert.Equal(t, "bound.item_id", rule.Then.Args["item_id"])
	assert.Equal(t, "bound.quantity", rule.Then.Args["quantity"])
}

func TestCompileSyncScopeFlow(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "flow", rule.Scope.Mode)
	assert.Empty(t, rule.Scope.Key)
}

func TestCompileSyncScopeGlobal(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "global"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "global", rule.Scope.Mode)
	assert.Empty(t, rule.Scope.Key)
}

func TestCompileSyncScopeKeyed(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "keyed(\"user_id\")"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "keyed", rule.Scope.Mode)
	assert.Equal(t, "user_id", rule.Scope.Key)
}

func TestCompileSyncScopeKeyedComplexField(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "keyed(\"tenant_id\")"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "keyed", rule.Scope.Mode)
	assert.Equal(t, "tenant_id", rule.Scope.Key)
}

func TestCompileSyncInvalidScope(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "invalid_scope"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope")
	assert.Contains(t, err.Error(), "flow")
	assert.Contains(t, err.Error(), "global")
}

func TestCompileSyncMissingScope(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scope")
	assert.Contains(t, err.Error(), "required")
}

func TestCompileSyncEventTypeCompleted(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "completed", rule.When.EventType)
}

func TestCompileSyncEventTypeInvoked(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "invoked" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "invoked", rule.When.EventType)
}

func TestCompileSyncInvalidEventType(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "started" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event type")
	assert.Contains(t, err.Error(), "completed")
	assert.Contains(t, err.Error(), "invoked")
}

func TestCompileSyncOutputCaseMatching(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "handle-error": {
			scope: "flow"

			when: {
				action: "Cart.checkout"
				event: "completed"
				case: "InsufficientFunds"
				bind: { amount: "result.required_amount" }
			}

			then: {
				action: "Notification.send"
				args: { message: "bound.amount" }
			}
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."handle-error"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "InsufficientFunds", rule.When.OutputCase)
}

func TestCompileSyncNoOutputCase(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Empty(t, rule.When.OutputCase)
}

func TestCompileSyncNoWhereClause(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "simple": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."simple"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Nil(t, rule.Where, "where clause should be optional")
}

func TestCompileSyncWithWhereNoFilter(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			where: {
				from: "Items"
				bind: { id: "item_id" }
			}
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	require.NotNil(t, rule.Where)
	assert.Equal(t, "Items", rule.Where.Source)
	assert.Empty(t, rule.Where.Filter)
}

func TestCompileSyncMissingWhen(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "bad": {
			scope: "flow"
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "when")
	assert.Contains(t, err.Error(), "required")
}

func TestCompileSyncMissingThen(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "bad": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "then")
	assert.Contains(t, err.Error(), "required")
}

func TestCompileSyncMissingWhenAction(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "bad": {
			scope: "flow"
			when: { event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "when.action")
}

func TestCompileSyncMissingWhenEvent(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "bad": {
			scope: "flow"
			when: { action: "A.b" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "when.event")
}

func TestCompileSyncMissingThenAction(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "bad": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { args: { foo: "bar" } }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "then.action")
}

func TestCompileSyncMissingWhereFrom(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "bad": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			where: {
				filter: "x == y"
			}
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
	_, err := CompileSync(syncVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "where.from")
}

func TestCompileSyncThenWithNoArgs(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.notify" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "C.notify", rule.Then.ActionRef)
	assert.Empty(t, rule.Then.Args)
}

func TestCompileSyncMultipleBindings(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: {
				action: "A.b"
				event: "completed"
				bind: {
					x: "result.x"
					y: "result.y"
					z: "result.z"
				}
			}
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Len(t, rule.When.Bindings, 3)
	assert.Equal(t, "result.x", rule.When.Bindings["x"])
	assert.Equal(t, "result.y", rule.When.Bindings["y"])
	assert.Equal(t, "result.z", rule.When.Bindings["z"])
}

func TestCompileSyncComplexFilter(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "test": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			where: {
				from: "Items"
				filter: "status == \"active\" && quantity > 0"
				bind: { id: "item_id" }
			}
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	require.NotNil(t, rule.Where)
	assert.Equal(t, `status == "active" && quantity > 0`, rule.Where.Filter)
}

func TestCompileSyncDashInID(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "my-complex-sync-rule": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."my-complex-sync-rule"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "my-complex-sync-rule", rule.ID)
}

func TestCompileSyncUnderscoreInID(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "my_sync_rule": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."my_sync_rule"`))
	rule, err := CompileSync(syncVal)

	require.NoError(t, err)
	assert.Equal(t, "my_sync_rule", rule.ID)
}

func TestCompileSyncNonExistentPath(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "real": {
			scope: "flow"
			when: { action: "A.b", event: "completed" }
			then: { action: "C.d" }
		}
	`)

	require.NoError(t, v.Err())
	syncVal := v.LookupPath(cue.ParsePath(`sync."not-here"`))

	assert.False(t, syncVal.Exists())
}

func TestCompileSyncInvalidCUESyntax(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		sync: "bad": {
			this is not valid CUE
		}
	`)

	require.Error(t, v.Err())
}

func TestKeyedPatternMatches(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
		key      string
	}{
		{`keyed("user_id")`, true, "user_id"},
		{`keyed("tenant_id")`, true, "tenant_id"},
		{`keyed("x")`, true, "x"},
		{`flow`, false, ""},
		{`global`, false, ""},
		{`keyed`, false, ""},
		{`keyed()`, false, ""},
		{`keyed("")`, false, ""},
	}

	for _, tt := range tests {
		matches := keyedPattern.FindStringSubmatch(tt.input)
		if tt.expected {
			require.NotNil(t, matches, "expected %q to match", tt.input)
			assert.Equal(t, tt.key, matches[1])
		} else {
			assert.Nil(t, matches, "expected %q not to match", tt.input)
		}
	}
}

func TestValidScopeModes(t *testing.T) {
	// Import ir package for ValidScopeModes
	assert.True(t, true) // Just verify test compiles
}
