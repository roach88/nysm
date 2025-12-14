# Story 3.6: Flow Token Propagation

Status: done

## Story

As a **developer building the engine**,
I want **flow tokens propagated through all invocations and completions**,
So that **sync rules can scope by flow**.

## Acceptance Criteria

1. **generateInvocation function in `internal/engine/engine.go`**
   ```go
   func (e *Engine) generateInvocation(flowToken string, then ir.ThenClause, bindings ir.IRObject) (ir.Invocation, error) {
       args, err := e.resolveArgs(then.Args, bindings)
       if err != nil {
           return ir.Invocation{}, fmt.Errorf("resolve args: %w", err)
       }

       inv := ir.Invocation{
           FlowToken:       flowToken,  // Inherited from triggering completion
           ActionURI:       then.Action,
           Args:            args,
           Seq:             e.clock.Next(),
           SecurityContext: e.currentSecurityContext(),
           SpecHash:        e.specHash,
           EngineVersion:   ir.EngineVersion,
           IRVersion:       ir.IRVersion,
       }

       // Compute content-addressed ID AFTER all fields set
       inv.ID = ir.InvocationID(inv)

       return inv, nil
   }
   ```

2. **Flow token inherited from triggering completion, never generated mid-flow**
   - Given completion with `FlowToken: "flow-abc"`
   - When sync rule fires and generates new invocation
   - Then new invocation has `FlowToken: "flow-abc"` (same as triggering completion)
   - Flow token is NEVER generated during sync rule execution
   - Flow token is ONLY generated for user-initiated requests (Story 3.5)

3. **All generated invocations use same flow token as triggering completion**
   - Engine receives completion with flow token F
   - Sync rule matches completion
   - Where-clause produces N bindings
   - N invocations generated, ALL with flow token F
   - Multi-binding syncs create multiple invocations with identical flow token

4. **Flow token chain unbroken from root to leaf**
   - Initial request: user → invocation with flow token F (from Story 3.5)
   - That invocation completes → completion inherits flow token F
   - Completion triggers sync → new invocation inherits flow token F
   - Chain continues: all records in flow have identical flow token
   - Provenance edges maintain flow relationship

5. **Provenance edges link firings to invocations within same flow**
   - Sync firing recorded for completion (flow token F)
   - Invocation generated with flow token F
   - Provenance edge links firing → invocation
   - ReadProvenance shows complete chain within flow
   - ReadTriggered shows all invocations triggered within flow

6. **Comprehensive tests in `internal/engine/flow_propagation_test.go`**
   - Test: single invocation propagates flow token
   - Test: multi-binding sync creates multiple invocations with same flow
   - Test: chained syncs maintain flow token
   - Test: provenance chain preserves flow
   - Test: verify flow token never changes mid-flow

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-3.2** | Propagate flow tokens through all invocations/completions |
| **CP-7** | Flow token inherited, never generated mid-flow |
| **HIGH-1** | Flow token scoping modes (Story 3.7 implements filtering) |

## Tasks / Subtasks

- [ ] Task 1: Implement generateInvocation function (AC: #1, #2, #3)
  - [ ] 1.1 Create `generateInvocation(flowToken, then, bindings)` in `internal/engine/engine.go`
  - [ ] 1.2 Accept flowToken as parameter (inherited from completion)
  - [ ] 1.3 Resolve args from then-clause and bindings
  - [ ] 1.4 Set FlowToken field to inherited value
  - [ ] 1.5 Generate sequence number via logical clock
  - [ ] 1.6 Compute content-addressed ID via `ir.InvocationID()`
  - [ ] 1.7 Return error if arg resolution fails

- [ ] Task 2: Integrate with sync rule execution (AC: #3, #4)
  - [ ] 2.1 Update `fireSyncRule` to call `generateInvocation`
  - [ ] 2.2 Pass completion.FlowToken to generateInvocation
  - [ ] 2.3 Handle multi-binding scenarios (loop over bindings)
  - [ ] 2.4 Write generated invocation to store
  - [ ] 2.5 Record provenance edge (firing → invocation)

- [ ] Task 3: Verify flow token propagation chain (AC: #4, #5)
  - [ ] 3.1 Trace flow token from root invocation
  - [ ] 3.2 Verify completion inherits flow token from invocation
  - [ ] 3.3 Verify generated invocations inherit flow token from completion
  - [ ] 3.4 Verify provenance edges maintain flow relationship
  - [ ] 3.5 Add logging for flow token propagation (slog)

- [ ] Task 4: Write comprehensive tests (AC: #6)
  - [ ] 4.1 Create `internal/engine/flow_propagation_test.go`
  - [ ] 4.2 Test single invocation flow token propagation
  - [ ] 4.3 Test multi-binding with same flow token
  - [ ] 4.4 Test chained syncs (3+ levels deep)
  - [ ] 4.5 Test provenance chain integrity
  - [ ] 4.6 Test that flow token never changes within flow

- [ ] Task 5: Verify integration with existing components
  - [ ] 5.1 Verify WriteInvocation handles inherited flow tokens
  - [ ] 5.2 Verify ReadFlow queries work with propagated tokens
  - [ ] 5.3 Verify crash recovery maintains flow token chains
  - [ ] 5.4 Verify provenance queries work with flow tokens

## Dev Notes

### Critical Implementation Details

**Flow Token Lifecycle**

```
USER REQUEST
    ↓
[Story 3.5: Generate flow token F via UUIDv7]
    ↓
Initial Invocation (flow_token: F)
    ↓
[Action executes]
    ↓
Completion (flow_token: F, inherits from invocation)
    ↓
[Story 3.6: Sync rule fires]
    ↓
generateInvocation(flowToken: F, ...)  ← Flow token INHERITED
    ↓
New Invocation (flow_token: F)  ← SAME flow token
    ↓
[Action executes]
    ↓
Completion (flow_token: F)
    ↓
[Repeat: next sync rule fires with flow_token: F]
```

**Flow Token is NEVER Generated Mid-Flow**

```go
// ❌ WRONG: Generating new flow token mid-flow breaks chain
func (e *Engine) generateInvocation(then ir.ThenClause, bindings ir.IRObject) ir.Invocation {
    newFlowToken := e.flowGen.Generate() // ❌ NEVER do this!
    return ir.Invocation{
        FlowToken: newFlowToken, // ❌ Breaks flow isolation
        // ...
    }
}

// ✅ CORRECT: Flow token inherited from triggering completion
func (e *Engine) generateInvocation(flowToken string, then ir.ThenClause, bindings ir.IRObject) (ir.Invocation, error) {
    // flowToken parameter comes from completion.FlowToken
    inv := ir.Invocation{
        FlowToken: flowToken, // ✅ Inherited, never generated
        // ...
    }
    return inv, nil
}
```

**Multi-Binding with Same Flow Token**

```go
// Scenario: One completion triggers 3 invocations (multi-binding)
// ALL invocations get SAME flow token

comp := ir.Completion{
    FlowToken:  "flow-abc-123",
    OutputCase: "Success",
    // ...
}

// Where-clause produces 3 bindings
bindings := []ir.IRObject{
    {"product": ir.IRString("widget")},
    {"product": ir.IRString("gadget")},
    {"product": ir.IRString("doohickey")},
}

// Generate 3 invocations, all with flow_token: "flow-abc-123"
for _, binding := range bindings {
    inv, err := e.generateInvocation(comp.FlowToken, sync.Then, binding)
    // inv.FlowToken == "flow-abc-123" for ALL invocations
    // This is CORRECT - all invocations in same flow
}
```

**Flow Token Chain Integrity**

```go
// Flow token must remain constant throughout entire flow
// Provenance chain verifies this

// Root invocation (user-initiated)
rootInv := ir.Invocation{
    FlowToken: "flow-root-xyz",
    ActionURI: "Order.Create",
    // ...
}

// Completion inherits flow token
comp1 := ir.Completion{
    InvocationID: rootInv.ID,
    FlowToken:    rootInv.FlowToken, // Same: "flow-root-xyz"
    // ...
}

// Generated invocation inherits flow token
inv2, _ := e.generateInvocation(comp1.FlowToken, then, bindings)
// inv2.FlowToken == "flow-root-xyz"

// Next completion inherits flow token
comp2 := ir.Completion{
    InvocationID: inv2.ID,
    FlowToken:    inv2.FlowToken, // Still: "flow-root-xyz"
    // ...
}

// INVARIANT: All records in flow have identical flow_token
// This enables flow-scoped queries and sync matching (Story 3.7)
```

### Function Signatures

**generateInvocation Implementation**

```go
// internal/engine/engine.go

// generateInvocation creates a new invocation from a then-clause.
// The flow token is INHERITED from the triggering completion, never generated.
//
// Parameters:
//   - flowToken: Flow token from the completion that triggered this sync
//   - then: Then-clause from sync rule (action + arg templates)
//   - bindings: Variable bindings from when-clause and where-clause
//
// Returns:
//   - Invocation with inherited flow token and computed content-addressed ID
//   - Error if arg resolution fails
//
// CRITICAL: Flow token is a PARAMETER, not generated. This ensures flow
// token chain remains unbroken from root to leaf.
func (e *Engine) generateInvocation(flowToken string, then ir.ThenClause, bindings ir.IRObject) (ir.Invocation, error) {
    // Validate flow token
    if flowToken == "" {
        return ir.Invocation{}, fmt.Errorf("flow token is required")
    }

    // Resolve args from then-clause templates and bindings
    args, err := e.resolveArgs(then.Args, bindings)
    if err != nil {
        return ir.Invocation{}, fmt.Errorf("resolve args for action %s: %w", then.Action, err)
    }

    // Build invocation with inherited flow token
    inv := ir.Invocation{
        FlowToken:       flowToken, // INHERITED from completion
        ActionURI:       then.Action,
        Args:            args,
        Seq:             e.clock.Next(),
        SecurityContext: e.currentSecurityContext(),
        SpecHash:        e.specHash,
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    // Compute content-addressed ID (includes flow token in hash)
    inv.ID = ir.InvocationID(inv)

    return inv, nil
}
```

**resolveArgs Helper Function**

```go
// resolveArgs substitutes binding variables into then-clause arg templates.
// Example:
//   then.Args = { "product_id": "${bound.product}", "qty": "${bound.quantity}" }
//   bindings  = { "product": "widget", "quantity": 5 }
//   result    = { "product_id": "widget", "qty": 5 }
func (e *Engine) resolveArgs(argTemplates ir.IRObject, bindings ir.IRObject) (ir.IRObject, error) {
    resolved := make(ir.IRObject)

    for key, template := range argTemplates {
        // If template is a binding reference (e.g., "${bound.product}")
        // then substitute with actual value
        // Otherwise use template value directly
        val, err := e.substituteBindings(template, bindings)
        if err != nil {
            return nil, fmt.Errorf("substitute binding for key %s: %w", key, err)
        }
        resolved[key] = val
    }

    return resolved, nil
}

// substituteBindings replaces binding references with actual values.
// This is a simplified implementation - full version in Story 3.8.
func (e *Engine) substituteBindings(template ir.IRValue, bindings ir.IRObject) (ir.IRValue, error) {
    // For Story 3.6, we assume simple binding references
    // Story 3.8 implements full template expression evaluation
    switch v := template.(type) {
    case ir.IRString:
        // Check if string is a binding reference (e.g., "${bound.product}")
        if strings.HasPrefix(string(v), "${bound.") && strings.HasSuffix(string(v), "}") {
            bindingName := strings.TrimSuffix(strings.TrimPrefix(string(v), "${bound."), "}")
            if val, ok := bindings[bindingName]; ok {
                return val, nil
            }
            return nil, fmt.Errorf("binding %s not found", bindingName)
        }
        return v, nil // Not a binding reference, use as-is
    default:
        // Non-string values pass through
        return template, nil
    }
}
```

**Integration with fireSyncRule**

```go
// fireSyncRule executes a sync rule for a specific binding set.
// This is called by ProcessCompletion for each matching sync/binding pair.
func (e *Engine) fireSyncRule(ctx context.Context, sync ir.SyncRule, comp ir.Completion, bindings ir.IRObject) error {
    // Compute binding hash for idempotency check
    // NOTE: BindingHash returns (string, error) per Story 1.5
    bindingHash, err := ir.BindingHash(bindings)
    if err != nil {
        return fmt.Errorf("compute binding hash: %w", err)
    }

    // Check if this sync already fired for these bindings (idempotency)
    // NOTE: Unified API name is HasFiring (not SyncFiringExists) per Story 5.1
    exists, err := e.store.HasFiring(ctx, comp.ID, sync.ID, bindingHash)
    if err != nil {
        return fmt.Errorf("check HasFiring: %w", err)
    }
    if exists {
        // Already fired - skip (idempotent replay)
        return nil
    }

    // Generate invocation with INHERITED flow token
    inv, err := e.generateInvocation(comp.FlowToken, sync.Then, bindings)
    if err != nil {
        return fmt.Errorf("generate invocation: %w", err)
    }

    // Write invocation to store
    if err := e.store.WriteInvocation(ctx, inv); err != nil {
        return fmt.Errorf("write invocation: %w", err)
    }

    // Record sync firing
    firing := ir.SyncFiring{
        CompletionID: comp.ID,
        SyncID:       sync.ID,
        BindingHash:  bindingHash,
        Seq:          e.clock.Next(),
    }
    result, err := e.store.WriteSyncFiring(ctx, firing)
    if err != nil {
        return fmt.Errorf("write sync firing: %w", err)
    }
    firing.ID = result.LastInsertId()

    // Record provenance edge: firing → invocation
    edge := ir.ProvenanceEdge{
        SyncFiringID: firing.ID,
        InvocationID: inv.ID,
    }
    if err := e.store.WriteProvenanceEdge(ctx, edge); err != nil {
        return fmt.Errorf("write provenance edge: %w", err)
    }

    // Log flow token propagation
    slog.Info("sync fired",
        "sync_id", sync.ID,
        "completion_id", comp.ID,
        "invocation_id", inv.ID,
        "flow_token", comp.FlowToken,
        "seq", inv.Seq,
    )

    return nil
}
```

### Test Examples

**Test Single Invocation Flow Token Propagation**

```go
func TestGenerateInvocation_FlowTokenPropagation(t *testing.T) {
    engine := setupTestEngine(t)

    flowToken := "flow-test-123"
    then := ir.ThenClause{
        Action: "Inventory.ReserveStock",
        Args: ir.IRObject{
            "product_id": ir.IRString("${bound.product}"),
            "quantity":   ir.IRInt(5),
        },
    }
    bindings := ir.IRObject{
        "product": ir.IRString("widget"),
    }

    // Generate invocation
    inv, err := engine.generateInvocation(flowToken, then, bindings)
    require.NoError(t, err)

    // Verify flow token inherited
    assert.Equal(t, flowToken, inv.FlowToken, "flow token must be inherited from completion")
    assert.Equal(t, "Inventory.ReserveStock", string(inv.ActionURI))
    assert.Equal(t, ir.IRString("widget"), inv.Args["product_id"])
    assert.Equal(t, ir.IRInt(5), inv.Args["quantity"])
}
```

**Test Multi-Binding with Same Flow Token**

```go
func TestFireSyncRule_MultiBindingSameFlow(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // Create root invocation with flow token
    flowToken := "flow-multi-123"
    rootInv := testInvocation("BatchJob.Complete", 1)
    rootInv.FlowToken = flowToken
    rootInv.ID = ir.InvocationID(rootInv)
    require.NoError(t, engine.store.WriteInvocation(ctx, rootInv))

    // Completion triggers sync with 3 bindings
    comp := testCompletion(rootInv.ID, "Success", 2)
    comp.FlowToken = flowToken
    comp.Result = ir.IRObject{
        "items": ir.IRArray{
            ir.IRObject{"product": ir.IRString("widget")},
            ir.IRObject{"product": ir.IRString("gadget")},
            ir.IRObject{"product": ir.IRString("doohickey")},
        },
    }
    comp.ID = ir.CompletionID(comp)
    require.NoError(t, engine.store.WriteCompletion(ctx, comp))

    // Sync rule with multi-binding where-clause
    sync := ir.SyncRule{
        ID: "sync-process-items",
        When: ir.WhenClause{
            Action:     "BatchJob.Complete",
            Event:      "completed",
            OutputCase: ptr("Success"),
        },
        // Where-clause would query items (simplified for test)
        Then: ir.ThenClause{
            Action: "Item.Process",
            Args: ir.IRObject{
                "product": ir.IRString("${bound.product}"),
            },
        },
    }

    // Simulate multi-binding where-clause results
    bindings := []ir.IRObject{
        {"product": ir.IRString("widget")},
        {"product": ir.IRString("gadget")},
        {"product": ir.IRString("doohickey")},
    }

    var generatedInvs []ir.Invocation
    for _, binding := range bindings {
        // Fire sync for each binding
        err := engine.fireSyncRule(ctx, sync, comp, binding)
        require.NoError(t, err)

        // Verify invocation created with same flow token
        inv, err := engine.generateInvocation(comp.FlowToken, sync.Then, binding)
        require.NoError(t, err)
        generatedInvs = append(generatedInvs, inv)
    }

    // Verify ALL invocations have SAME flow token
    for i, inv := range generatedInvs {
        assert.Equal(t, flowToken, inv.FlowToken,
            "invocation %d must have same flow token as triggering completion", i)
    }

    // Verify 3 invocations created
    assert.Len(t, generatedInvs, 3)

    // Verify all invocations queryable by flow token
    flowData, err := engine.store.ReadFlow(ctx, flowToken)
    require.NoError(t, err)
    // Should include: root invocation + completion + 3 generated invocations
    assert.Len(t, flowData.Invocations, 4, "root + 3 generated")
    assert.Len(t, flowData.Completions, 1, "root completion")
}
```

**Test Chained Syncs Maintain Flow Token**

```go
func TestFlowTokenChain_ThreeLevelsDeep(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    flowToken := "flow-chain-xyz"

    // Level 1: User initiates Order.Create
    inv1 := testInvocation("Order.Create", 1)
    inv1.FlowToken = flowToken
    inv1.ID = ir.InvocationID(inv1)
    engine.store.WriteInvocation(ctx, inv1)

    // Level 1 completion
    comp1 := testCompletion(inv1.ID, "Success", 2)
    comp1.FlowToken = flowToken
    comp1.ID = ir.CompletionID(comp1)
    engine.store.WriteCompletion(ctx, comp1)

    // Level 2: Sync generates Inventory.ReserveStock
    sync1 := testSyncRule("sync-reserve", "Order.Create", "Inventory.ReserveStock")
    inv2, err := engine.generateInvocation(comp1.FlowToken, sync1.Then, ir.IRObject{})
    require.NoError(t, err)
    assert.Equal(t, flowToken, inv2.FlowToken, "level 2 invocation must inherit flow token")
    engine.store.WriteInvocation(ctx, inv2)

    // Level 2 completion
    comp2 := testCompletion(inv2.ID, "Success", 4)
    comp2.FlowToken = flowToken
    comp2.ID = ir.CompletionID(comp2)
    engine.store.WriteCompletion(ctx, comp2)

    // Level 3: Sync generates Notification.Send
    sync2 := testSyncRule("sync-notify", "Inventory.ReserveStock", "Notification.Send")
    inv3, err := engine.generateInvocation(comp2.FlowToken, sync2.Then, ir.IRObject{})
    require.NoError(t, err)
    assert.Equal(t, flowToken, inv3.FlowToken, "level 3 invocation must inherit flow token")
    engine.store.WriteInvocation(ctx, inv3)

    // Verify entire chain has same flow token
    flowData, err := engine.store.ReadFlow(ctx, flowToken)
    require.NoError(t, err)

    assert.Len(t, flowData.Invocations, 3, "3 invocations in chain")
    assert.Len(t, flowData.Completions, 2, "2 completions so far")

    // Verify flow token constant throughout chain
    for i, inv := range flowData.Invocations {
        assert.Equal(t, flowToken, inv.FlowToken,
            "invocation %d must have flow token %s", i, flowToken)
    }
    for i, comp := range flowData.Completions {
        assert.Equal(t, flowToken, comp.FlowToken,
            "completion %d must have flow token %s", i, flowToken)
    }
}
```

**Test Provenance Chain Preserves Flow**

```go
func TestProvenance_FlowTokenChain(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    flowToken := "flow-prov-abc"

    // Build chain: inv1 → comp1 → (sync) → inv2 → comp2 → (sync) → inv3
    inv1 := testInvocation("Action1", 1)
    inv1.FlowToken = flowToken
    inv1.ID = ir.InvocationID(inv1)
    engine.store.WriteInvocation(ctx, inv1)

    comp1 := testCompletion(inv1.ID, "Success", 2)
    comp1.FlowToken = flowToken
    comp1.ID = ir.CompletionID(comp1)
    engine.store.WriteCompletion(ctx, comp1)

    // Fire sync1
    sync1 := testSyncRule("sync1", "Action1", "Action2")
    bindings1 := ir.IRObject{"key": ir.IRString("value1")}
    err := engine.fireSyncRule(ctx, sync1, comp1, bindings1)
    require.NoError(t, err)

    // Find inv2 (generated by sync1)
    triggered1, err := engine.store.ReadTriggered(ctx, comp1.ID)
    require.NoError(t, err)
    require.Len(t, triggered1, 1)
    inv2 := triggered1[0]
    assert.Equal(t, flowToken, inv2.FlowToken, "inv2 must have same flow token")

    comp2 := testCompletion(inv2.ID, "Success", 4)
    comp2.FlowToken = flowToken
    comp2.ID = ir.CompletionID(comp2)
    engine.store.WriteCompletion(ctx, comp2)

    // Fire sync2
    sync2 := testSyncRule("sync2", "Action2", "Action3")
    bindings2 := ir.IRObject{"key": ir.IRString("value2")}
    err = engine.fireSyncRule(ctx, sync2, comp2, bindings2)
    require.NoError(t, err)

    // Find inv3 (generated by sync2)
    triggered2, err := engine.store.ReadTriggered(ctx, comp2.ID)
    require.NoError(t, err)
    require.Len(t, triggered2, 1)
    inv3 := triggered2[0]
    assert.Equal(t, flowToken, inv3.FlowToken, "inv3 must have same flow token")

    // Trace provenance backward from inv3 to inv1
    prov3, err := engine.store.ReadProvenance(ctx, inv3.ID)
    require.NoError(t, err)
    assert.Equal(t, comp2.ID, prov3[0].CompletionID, "inv3 caused by comp2")

    prov2, err := engine.store.ReadProvenance(ctx, inv2.ID)
    require.NoError(t, err)
    assert.Equal(t, comp1.ID, prov2[0].CompletionID, "inv2 caused by comp1")

    prov1, err := engine.store.ReadProvenance(ctx, inv1.ID)
    require.NoError(t, err)
    assert.Empty(t, prov1, "inv1 is root (user-initiated)")

    // Verify all records in provenance chain have same flow token
    allInvs := []ir.Invocation{inv1, inv2, inv3}
    for i, inv := range allInvs {
        assert.Equal(t, flowToken, inv.FlowToken,
            "invocation %d in provenance chain must have flow token %s", i, flowToken)
    }
}
```

**Test Flow Token Never Changes Mid-Flow**

```go
func TestFlowToken_NeverGeneratedMidFlow(t *testing.T) {
    engine := setupTestEngine(t)

    // Completion with flow token
    originalFlow := "flow-original-123"
    comp := ir.Completion{
        FlowToken:  originalFlow,
        OutputCase: "Success",
        // ...
    }

    then := ir.ThenClause{
        Action: "NextAction",
        Args:   ir.IRObject{},
    }
    bindings := ir.IRObject{}

    // Generate invocation - must use completion's flow token
    inv, err := engine.generateInvocation(comp.FlowToken, then, bindings)
    require.NoError(t, err)

    // CRITICAL: Flow token MUST match completion's flow token
    assert.Equal(t, originalFlow, inv.FlowToken,
        "flow token must be inherited, never generated")

    // Verify flow token is NOT a new UUIDv7
    // (If it were generated, it would be different from originalFlow)
    assert.NotContains(t, inv.FlowToken, "new-flow",
        "flow token must not be newly generated")
}
```

**Test Missing Flow Token Error**

```go
func TestGenerateInvocation_MissingFlowToken(t *testing.T) {
    engine := setupTestEngine(t)

    then := ir.ThenClause{
        Action: "Test.Action",
        Args:   ir.IRObject{},
    }
    bindings := ir.IRObject{}

    // Attempt to generate invocation with empty flow token
    _, err := engine.generateInvocation("", then, bindings)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "flow token is required")
}
```

**Test Arg Resolution with Bindings**

```go
func TestGenerateInvocation_ArgResolution(t *testing.T) {
    engine := setupTestEngine(t)

    flowToken := "flow-args-test"
    then := ir.ThenClause{
        Action: "Order.Process",
        Args: ir.IRObject{
            "order_id":   ir.IRString("${bound.order_id}"),
            "product":    ir.IRString("${bound.product_name}"),
            "quantity":   ir.IRInt(10), // Not a binding reference
            "priority":   ir.IRString("high"), // Not a binding reference
        },
    }
    bindings := ir.IRObject{
        "order_id":     ir.IRString("ord-123"),
        "product_name": ir.IRString("widget"),
    }

    inv, err := engine.generateInvocation(flowToken, then, bindings)
    require.NoError(t, err)

    // Verify args resolved correctly
    assert.Equal(t, ir.IRString("ord-123"), inv.Args["order_id"],
        "order_id should be resolved from bindings")
    assert.Equal(t, ir.IRString("widget"), inv.Args["product"],
        "product should be resolved from bindings")
    assert.Equal(t, ir.IRInt(10), inv.Args["quantity"],
        "quantity should pass through (not a binding reference)")
    assert.Equal(t, ir.IRString("high"), inv.Args["priority"],
        "priority should pass through (not a binding reference)")
}
```

**Test Arg Resolution Error (Missing Binding)**

```go
func TestGenerateInvocation_MissingBinding(t *testing.T) {
    engine := setupTestEngine(t)

    flowToken := "flow-missing-binding"
    then := ir.ThenClause{
        Action: "Test.Action",
        Args: ir.IRObject{
            "field": ir.IRString("${bound.nonexistent}"),
        },
    }
    bindings := ir.IRObject{
        // "nonexistent" binding not provided
    }

    _, err := engine.generateInvocation(flowToken, then, bindings)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "binding nonexistent not found")
}
```

### Flow Token Propagation Example

**Complete Flow Scenario**

```go
// Scenario: User creates order → reserves stock → sends notification
// ALL records have same flow token: "flow-order-abc"

// Step 1: User initiates request (Story 3.5 generates flow token)
flowToken := engine.flowGen.Generate() // UUIDv7: "flow-order-abc"
inv1 := ir.Invocation{
    FlowToken: flowToken,
    ActionURI: "Order.Create",
    Args: ir.IRObject{
        "product": ir.IRString("widget"),
        "qty":     ir.IRInt(5),
    },
    Seq: 1,
    // ...
}
inv1.ID = ir.InvocationID(inv1)
engine.store.WriteInvocation(ctx, inv1)

// Step 2: Order.Create completes
comp1 := ir.Completion{
    InvocationID: inv1.ID,
    FlowToken:    inv1.FlowToken, // Inherited: "flow-order-abc"
    OutputCase:   "Success",
    Result: ir.IRObject{
        "order_id": ir.IRString("ord-123"),
    },
    Seq: 2,
    // ...
}
comp1.ID = ir.CompletionID(comp1)
engine.store.WriteCompletion(ctx, comp1)

// Step 3: Sync rule fires: when Order.Create completes, reserve stock
sync1 := ir.SyncRule{
    ID: "sync-reserve-on-order",
    When: ir.WhenClause{
        Action:     "Order.Create",
        Event:      "completed",
        OutputCase: ptr("Success"),
        Bindings: map[string]string{
            "order_id": "order_id",
            "product":  "product",
            "qty":      "qty",
        },
    },
    Then: ir.ThenClause{
        Action: "Inventory.ReserveStock",
        Args: ir.IRObject{
            "order_id": ir.IRString("${bound.order_id}"),
            "product":  ir.IRString("${bound.product}"),
            "quantity": ir.IRInt("${bound.qty}"),
        },
    },
}

bindings1 := ir.IRObject{
    "order_id": ir.IRString("ord-123"),
    "product":  ir.IRString("widget"),
    "qty":      ir.IRInt(5),
}

// Generate invocation with INHERITED flow token
inv2, err := engine.generateInvocation(comp1.FlowToken, sync1.Then, bindings1)
// inv2.FlowToken == "flow-order-abc" (SAME as comp1)
inv2.ID = ir.InvocationID(inv2)
engine.store.WriteInvocation(ctx, inv2)

// Record sync firing and provenance
firing1 := ir.SyncFiring{
    CompletionID: comp1.ID,
    SyncID:       sync1.ID,
    BindingHash:  ir.BindingHash(bindings1),
    Seq:          3,
}
result1, _ := engine.store.WriteSyncFiring(ctx, firing1)
firing1.ID = result1.LastInsertId()

edge1 := ir.ProvenanceEdge{
    SyncFiringID: firing1.ID,
    InvocationID: inv2.ID,
}
engine.store.WriteProvenanceEdge(ctx, edge1)

// Step 4: Inventory.ReserveStock completes
comp2 := ir.Completion{
    InvocationID: inv2.ID,
    FlowToken:    inv2.FlowToken, // Inherited: "flow-order-abc"
    OutputCase:   "Success",
    Result: ir.IRObject{
        "reservation_id": ir.IRString("res-456"),
    },
    Seq: 4,
    // ...
}
comp2.ID = ir.CompletionID(comp2)
engine.store.WriteCompletion(ctx, comp2)

// Step 5: Sync rule fires: when ReserveStock completes, notify user
sync2 := ir.SyncRule{
    ID: "sync-notify-on-reserve",
    When: ir.WhenClause{
        Action:     "Inventory.ReserveStock",
        Event:      "completed",
        OutputCase: ptr("Success"),
    },
    Then: ir.ThenClause{
        Action: "Notification.Send",
        Args: ir.IRObject{
            "order_id": ir.IRString("${bound.order_id}"),
            "message":  ir.IRString("Stock reserved"),
        },
    },
}

bindings2 := ir.IRObject{
    "order_id": ir.IRString("ord-123"),
}

// Generate invocation with INHERITED flow token
inv3, err := engine.generateInvocation(comp2.FlowToken, sync2.Then, bindings2)
// inv3.FlowToken == "flow-order-abc" (SAME as comp2, inv2, comp1, inv1)
inv3.ID = ir.InvocationID(inv3)
engine.store.WriteInvocation(ctx, inv3)

// Record sync firing and provenance
firing2 := ir.SyncFiring{
    CompletionID: comp2.ID,
    SyncID:       sync2.ID,
    BindingHash:  ir.BindingHash(bindings2),
    Seq:          5,
}
result2, _ := engine.store.WriteSyncFiring(ctx, firing2)
firing2.ID = result2.LastInsertId()

edge2 := ir.ProvenanceEdge{
    SyncFiringID: firing2.ID,
    InvocationID: inv3.ID,
}
engine.store.WriteProvenanceEdge(ctx, edge2)

// VERIFICATION: All records have SAME flow token
flowData, _ := engine.store.ReadFlow(ctx, flowToken)
// flowData.Invocations = [inv1, inv2, inv3] - all with flow_token "flow-order-abc"
// flowData.Completions = [comp1, comp2] - all with flow_token "flow-order-abc"

// PROVENANCE CHAIN:
// inv1 (root) → comp1 → [sync1] → inv2 → comp2 → [sync2] → inv3
// All records: flow_token = "flow-order-abc"
```

### Logging Pattern

```go
// Log flow token propagation for debugging
slog.Info("invocation generated",
    "invocation_id", inv.ID,
    "action_uri", inv.ActionURI,
    "flow_token", inv.FlowToken,
    "seq", inv.Seq,
    "triggered_by_completion", comp.ID,
    "sync_id", sync.ID,
)
```

### File List

Files to modify:

1. `internal/engine/engine.go` - Add generateInvocation, resolveArgs, substituteBindings
2. `internal/engine/engine.go` - Update fireSyncRule to use generateInvocation

Files to create:

1. `internal/engine/flow_propagation_test.go` - Comprehensive flow token propagation tests

Files that must exist (from previous stories):

1. `internal/ir/types.go` - Invocation, Completion, SyncRule, ThenClause types
2. `internal/ir/hash.go` - InvocationID, CompletionID, BindingHash functions
3. `internal/store/write.go` - WriteInvocation, WriteSyncFiring, WriteProvenanceEdge
4. `internal/store/read.go` - ReadFlow, ReadProvenance, ReadTriggered
5. `internal/engine/clock.go` - LogicalClock interface
6. Story 3.5 - Flow token generation (UUIDv7 generator)

### Story Completion Checklist

- [ ] generateInvocation function implemented
- [ ] Flow token accepted as parameter (inherited, not generated)
- [ ] resolveArgs function implemented
- [ ] substituteBindings function implemented (simplified version)
- [ ] fireSyncRule calls generateInvocation with completion.FlowToken
- [ ] Content-addressed ID computed via InvocationID()
- [ ] Provenance edges recorded for generated invocations
- [ ] Test: single invocation flow token propagation
- [ ] Test: multi-binding with same flow token
- [ ] Test: chained syncs maintain flow token
- [ ] Test: provenance chain preserves flow
- [ ] Test: flow token never changes mid-flow
- [ ] Test: arg resolution with bindings
- [ ] Test: missing binding error
- [ ] Test: missing flow token error
- [ ] Logging includes flow_token in all invocation generation
- [ ] `go vet ./internal/engine/...` passes
- [ ] `go test ./internal/engine/...` passes

### References

- [Source: docs/prd.md#FR-3.2] - Propagate flow tokens through all invocations/completions
- [Source: docs/architecture.md#Flow Token System] - Request correlation mechanism
- [Source: docs/epics.md#Story 3.6] - Story definition
- [Source: docs/epics.md#Story 3.5] - Flow token generation (prerequisite)
- [Source: Story 2.3] - WriteInvocation function
- [Source: Story 2.6] - Provenance edges
- [Source: Story 2.4] - ReadFlow queries
- [Source: Story 3.3] - When-clause matching
- [Source: Story 3.4] - Output case matching

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation

### Completion Notes

- Flow token is INHERITED from triggering completion, NEVER generated mid-flow
- generateInvocation accepts flowToken as parameter (not from generator)
- Multi-binding syncs create multiple invocations with IDENTICAL flow token
- Flow token chain remains unbroken from root (user request) to leaf (terminal action)
- Provenance edges link firings to invocations within same flow
- ReadFlow queries return all records with matching flow token
- This enables flow-scoped sync matching (Story 3.7)
- Flow isolation prevents concurrent requests from accidentally joining
- Content-addressed ID includes flow token in hash (from Story 1.5)
- Arg resolution uses binding substitution (full implementation in Story 3.8)
- Security context inherited from engine (same tenant/user throughout flow)
- Logging includes flow_token for correlation in production debugging
- Story implements FR-3.2 (propagate flow tokens through all records)
