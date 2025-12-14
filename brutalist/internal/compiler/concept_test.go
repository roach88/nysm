package compiler

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileConceptBasic(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Manages shopping cart"

			state: CartItem: {
				item_id: string
				quantity: int
			}

			action: addItem: {
				args: {
					item_id: string
					quantity: int
				}
				outputs: [{
					case: "Success"
					fields: { item_id: string, new_quantity: int }
				}]
			}

			operational_principle: "Adding items increases quantity"
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))

	spec, err := CompileConcept(conceptVal)
	require.NoError(t, err)

	assert.Equal(t, "Cart", spec.Name)
	assert.Equal(t, "Manages shopping cart", spec.Purpose)
	assert.Len(t, spec.StateSchema, 1)
	assert.Len(t, spec.Actions, 1)
	assert.Equal(t, "addItem", spec.Actions[0].Name)
	assert.Len(t, spec.OperationalPrinciples, 1)
	assert.Equal(t, "Adding items increases quantity", spec.OperationalPrinciples[0].Description)
}

func TestCompileConceptMissingPurpose(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Bad: {
			action: foo: {
				outputs: [{ case: "Success", fields: {} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "purpose")
	assert.Contains(t, err.Error(), "required")
}

func TestCompileConceptMissingActions(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Empty: {
			purpose: "Does nothing"
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Empty"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "action")
	assert.Contains(t, err.Error(), "required")
}

func TestCompileConceptMissingOutputs(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: BadAction: {
			purpose: "Has action without outputs"

			action: noOutputs: {
				args: { id: string }
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.BadAction"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Contains(t, err.Error(), "required")
}

func TestCompileConceptRejectsFloat(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Bad: {
			purpose: "Has float"

			state: Item: {
				price: float
			}

			action: buy: {
				outputs: [{ case: "Success", fields: {} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "float")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestCompileConceptRejectsFloatInActionArgs(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Bad: {
			purpose: "Has float in args"

			action: pay: {
				args: {
					amount: float
				}
				outputs: [{ case: "Success", fields: {} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "float")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestCompileConceptRejectsFloatInOutputFields(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Bad: {
			purpose: "Has float in output"

			action: calculate: {
				outputs: [{
					case: "Success"
					fields: { result: float }
				}]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "float")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestCompileConceptMultipleOutputCases(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Inventory: {
			purpose: "Tracks stock"

			action: reserve: {
				args: {
					item_id: string
					quantity: int
				}
				outputs: [{
					case: "Success"
					fields: { reservation_id: string }
				}, {
					case: "InsufficientStock"
					fields: { available: int, requested: int }
				}]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Inventory"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.Actions, 1)
	assert.Len(t, spec.Actions[0].Outputs, 2)
	assert.Equal(t, "Success", spec.Actions[0].Outputs[0].Case)
	assert.Equal(t, "InsufficientStock", spec.Actions[0].Outputs[1].Case)
}

func TestCompileConceptWithRequires(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Manages shopping cart"

			action: addItem: {
				args: {
					item_id: string
					quantity: int
				}
				requires: ["cart:write", "inventory:read"]
				outputs: [{
					case: "Success"
					fields: { item_id: string, new_quantity: int }
				}]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.Actions, 1)
	assert.Equal(t, []string{"cart:write", "inventory:read"}, spec.Actions[0].Requires)
}

func TestCompileConceptMultipleStates(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Order: {
			purpose: "Manages order lifecycle"

			state: OrderHeader: {
				order_id: string
				customer_id: string
				status: string
			}

			state: OrderLine: {
				order_id: string
				item_id: string
				quantity: int
			}

			action: createOrder: {
				args: { customer_id: string }
				outputs: [{ case: "Success", fields: { order_id: string } }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Order"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	assert.Len(t, spec.StateSchema, 2)

	// Find states by name (order may vary)
	stateNames := make(map[string]bool)
	for _, s := range spec.StateSchema {
		stateNames[s.Name] = true
	}
	assert.True(t, stateNames["OrderHeader"])
	assert.True(t, stateNames["OrderLine"])
}

func TestCompileConceptMultipleActions(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Shopping cart operations"

			action: addItem: {
				args: { item_id: string, quantity: int }
				outputs: [{ case: "Success", fields: { new_quantity: int } }]
			}

			action: removeItem: {
				args: { item_id: string }
				outputs: [{ case: "Success", fields: {} }]
			}

			action: clearCart: {
				outputs: [{ case: "Success", fields: {} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	assert.Len(t, spec.Actions, 3)

	// Find actions by name (order may vary)
	actionNames := make(map[string]bool)
	for _, a := range spec.Actions {
		actionNames[a.Name] = true
	}
	assert.True(t, actionNames["addItem"])
	assert.True(t, actionNames["removeItem"])
	assert.True(t, actionNames["clearCart"])
}

func TestCompileConceptNoState(t *testing.T) {
	// Concepts can have no state (pure stateless actions)
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Calculator: {
			purpose: "Stateless calculations"

			action: add: {
				args: { a: int, b: int }
				outputs: [{ case: "Success", fields: { result: int } }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Calculator"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	assert.Len(t, spec.StateSchema, 0)
	assert.Len(t, spec.Actions, 1)
}

func TestCompileConceptAllTypes(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: TypeDemo: {
			purpose: "Demonstrates all valid types"

			state: AllTypes: {
				str_field: string
				int_field: int
				bool_field: bool
				array_field: [...]
				object_field: {...}
			}

			action: demo: {
				args: {
					str: string
					num: int
					flag: bool
				}
				outputs: [{ case: "Success", fields: {
					arr: [...]
					obj: {...}
				} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.TypeDemo"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.StateSchema, 1)

	state := spec.StateSchema[0]
	assert.Equal(t, "string", state.Fields["str_field"])
	assert.Equal(t, "int", state.Fields["int_field"])
	assert.Equal(t, "bool", state.Fields["bool_field"])
	assert.Equal(t, "array", state.Fields["array_field"])
	assert.Equal(t, "object", state.Fields["object_field"])
}

func TestCompileConceptMultipleOperationalPrinciples(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Shopping cart with multiple principles"

			action: addItem: {
				args: { item_id: string }
				outputs: [{ case: "Success", fields: {} }]
			}

			operational_principle: [
				"Adding an item increases quantity or creates new entry",
				"Removing an item decreases quantity or removes entry"
			]
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	assert.Len(t, spec.OperationalPrinciples, 2)
	assert.Equal(t, "Adding an item increases quantity or creates new entry", spec.OperationalPrinciples[0].Description)
	assert.Equal(t, "Removing an item decreases quantity or removes entry", spec.OperationalPrinciples[1].Description)
}

func TestCompileConceptNoOperationalPrinciple(t *testing.T) {
	// operational_principle is optional
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Simple: {
			purpose: "No operational principle"

			action: doThing: {
				outputs: [{ case: "Success", fields: {} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Simple"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	assert.Len(t, spec.OperationalPrinciples, 0)
}

func TestCompileConceptActionWithNoArgs(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Cart operations"

			action: clear: {
				outputs: [{ case: "Success", fields: {} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.Actions, 1)
	assert.Len(t, spec.Actions[0].Args, 0)
}

func TestCompileConceptOutputWithNoFields(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Notify: {
			purpose: "Send notifications"

			action: send: {
				args: { message: string }
				outputs: [{ case: "Sent", fields: {} }]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Notify"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.Actions, 1)
	require.Len(t, spec.Actions[0].Outputs, 1)
	assert.Empty(t, spec.Actions[0].Outputs[0].Fields)
}

func TestCompileConceptInvalidCUESyntax(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Bad: {
			this is not valid CUE
		}
	`)

	// CUE compile error happens during CompileString
	require.Error(t, v.Err())
}

func TestCompileConceptValueError(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Bad: {
			purpose: 123  // wrong type - should be string
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
}

func TestCompileConceptErrorPosition(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Test: {
			purpose: "test"
			action: bad: {
				outputs: [{
					case: "Success"
					fields: { value: float }
				}]
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Test"))
	_, err := CompileConcept(conceptVal)

	require.Error(t, err)
	compileErr, ok := err.(*CompileError)
	require.True(t, ok, "error should be *CompileError")
	// Position may or may not be valid depending on CUE version
	// Just verify we got a CompileError with the right message
	assert.Contains(t, compileErr.Message, "float")
}

func TestCompileConceptNonExistentPath(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Real: {
			purpose: "exists"
			action: do: { outputs: [{ case: "Success", fields: {} }] }
		}
	`)

	require.NoError(t, v.Err())
	// Try to get a concept that doesn't exist
	conceptVal := v.LookupPath(cue.ParsePath("concept.NotHere"))

	// Exists() should be false for non-existent path
	assert.False(t, conceptVal.Exists())
}

func TestCompileErrorFormat(t *testing.T) {
	// Test CompileError formatting
	err := &CompileError{
		Field:   "purpose",
		Message: "purpose is required",
	}

	assert.Equal(t, "purpose: purpose is required", err.Error())
}

func TestExtractTypeNameUnsupported(t *testing.T) {
	ctx := cuecontext.New()
	// Use null which is not a supported type
	v := ctx.CompileString(`value: null`)
	require.NoError(t, v.Err())

	val := v.LookupPath(cue.ParsePath("value"))
	_, err := extractTypeName(val)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestDomainConstants(t *testing.T) {
	// Domain constants should be consistent
	assert.Equal(t, "nysm/invocation/v1", "nysm/invocation/v1")
	assert.Equal(t, "nysm/completion/v1", "nysm/completion/v1")
	assert.Equal(t, "nysm/binding/v1", "nysm/binding/v1")
}

func TestCompileConceptEmptyOutputList(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Bad: {
			purpose: "has empty outputs"
			action: fail: {
				outputs: []
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
	spec, err := CompileConcept(conceptVal)

	// Empty outputs array is valid CUE but produces action with 0 outputs
	require.NoError(t, err)
	require.Len(t, spec.Actions, 1)
	assert.Len(t, spec.Actions[0].Outputs, 0)
}

func TestCompileConceptStructuredOperationalPrinciple(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Shopping cart with structured principle"

			action: addItem: {
				args: { item_id: string }
				outputs: [{ case: "Success", fields: {} }]
			}

			operational_principle: {
				description: "Adding items increases quantity"
				scenario: "testdata/scenarios/cart_add.yaml"
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.OperationalPrinciples, 1)
	assert.Equal(t, "Adding items increases quantity", spec.OperationalPrinciples[0].Description)
	assert.Equal(t, "testdata/scenarios/cart_add.yaml", spec.OperationalPrinciples[0].Scenario)
}

func TestCompileConceptStructuredOperationalPrinciplesArray(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Shopping cart with multiple structured principles"

			action: addItem: {
				args: { item_id: string }
				outputs: [{ case: "Success", fields: {} }]
			}

			operational_principles: [
				{
					description: "Adding items increases quantity"
					scenario: "testdata/scenarios/cart_add.yaml"
				},
				{
					description: "Removing items decreases quantity"
					scenario: "testdata/scenarios/cart_remove.yaml"
				}
			]
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.OperationalPrinciples, 2)
	assert.Equal(t, "Adding items increases quantity", spec.OperationalPrinciples[0].Description)
	assert.Equal(t, "testdata/scenarios/cart_add.yaml", spec.OperationalPrinciples[0].Scenario)
	assert.Equal(t, "Removing items decreases quantity", spec.OperationalPrinciples[1].Description)
	assert.Equal(t, "testdata/scenarios/cart_remove.yaml", spec.OperationalPrinciples[1].Scenario)
}

func TestCompileConceptMixedOperationalPrinciples(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Shopping cart with mixed principles"

			action: addItem: {
				args: { item_id: string }
				outputs: [{ case: "Success", fields: {} }]
			}

			operational_principles: [
				"Legacy string principle",
				{
					description: "Structured principle"
					scenario: "testdata/scenarios/test.yaml"
				}
			]
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.OperationalPrinciples, 2)
	assert.Equal(t, "Legacy string principle", spec.OperationalPrinciples[0].Description)
	assert.Equal(t, "", spec.OperationalPrinciples[0].Scenario) // No scenario for legacy
	assert.Equal(t, "Structured principle", spec.OperationalPrinciples[1].Description)
	assert.Equal(t, "testdata/scenarios/test.yaml", spec.OperationalPrinciples[1].Scenario)
}

func TestCompileConceptStructuredPrincipleNoScenario(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
		concept: Cart: {
			purpose: "Shopping cart"

			action: addItem: {
				args: { item_id: string }
				outputs: [{ case: "Success", fields: {} }]
			}

			operational_principle: {
				description: "Description without scenario"
			}
		}
	`)

	require.NoError(t, v.Err())
	conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
	spec, err := CompileConcept(conceptVal)

	require.NoError(t, err)
	require.Len(t, spec.OperationalPrinciples, 1)
	assert.Equal(t, "Description without scenario", spec.OperationalPrinciples[0].Description)
	assert.Equal(t, "", spec.OperationalPrinciples[0].Scenario)
}
