package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// FlowTokenGenerator generates unique flow tokens for request correlation.
// Implemented by UUIDv7Generator (production) and FixedGenerator (tests).
// See flow.go for implementations (Story 3.5).
type FlowTokenGenerator interface {
	Generate() string
}

// Engine is the single-writer sync engine event loop.
//
// The engine processes events (invocations and completions) in FIFO order,
// evaluates sync rules, and generates follow-on invocations.
//
// CRITICAL: All mutations happen in the single-writer Run loop goroutine.
// External callers use Enqueue() to submit events for processing.
//
// Thread-safety model:
//   - Enqueue(): safe from any goroutine
//   - Run(): must be called from exactly one goroutine
//   - NewFlow(): safe from any goroutine (delegates to thread-safe generator)
//
// INVARIANTS:
//   - syncs slice order NEVER changes after construction (CRITICAL-3)
//   - Sync IDs within syncs are unique
//   - Evaluation is single-threaded for determinism

// DefaultMaxSteps is the default maximum number of steps per flow.
// This prevents runaway flows from consuming unbounded resources.
const DefaultMaxSteps = 1000

// Engine is defined with fields for event processing, sync rules,
// cycle detection, and quota enforcement.
type Engine struct {
	store         *store.Store
	clock         *Clock
	specs         []ir.ConceptSpec
	syncs         []ir.SyncRule // Sync rules in declaration order (CRITICAL-3)
	queue         *eventQueue
	flowGen       FlowTokenGenerator
	specHash      string // Hash of concept specs for versioning
	cycleDetector *CycleDetector

	// Quota enforcement (Story 5.4)
	maxSteps int                        // Maximum steps per flow (default: 1000)
	quotas   map[string]*QuotaEnforcer // Per-flow quota trackers

	// TODO (Story 4.1): Query IR compiler
	// compiler queryir.Compiler
}

// EngineOption allows configuration of engine parameters.
type EngineOption func(*Engine)

// WithMaxSteps sets the maximum steps quota per flow.
//
// Default: 1000 steps (DefaultMaxSteps)
// Use WithMaxSteps(5000) for flows with many distinct actions.
// Use WithMaxSteps(10) for testing quota enforcement.
func WithMaxSteps(maxSteps int) EngineOption {
	return func(e *Engine) {
		e.maxSteps = maxSteps
	}
}

// New creates an Engine with the given store, specs, syncs, and flow generator.
//
// The syncs slice must be in declaration order - this order is preserved for
// deterministic sync rule evaluation (CRITICAL-3).
//
// The syncs slice is copied to prevent external mutation from breaking
// the declaration order invariant.
//
// Options can be passed to configure the engine (e.g., WithMaxSteps).
func New(
	s *store.Store,
	specs []ir.ConceptSpec,
	syncs []ir.SyncRule,
	flowGen FlowTokenGenerator,
	opts ...EngineOption,
) *Engine {
	// Copy syncs to prevent external mutation (CRITICAL-3 protection)
	var syncsCopy []ir.SyncRule
	if syncs != nil {
		syncsCopy = make([]ir.SyncRule, len(syncs))
		copy(syncsCopy, syncs)
	}

	e := &Engine{
		store:         s,
		clock:         NewClock(),
		specs:         specs,
		syncs:         syncsCopy,
		queue:         newEventQueue(),
		flowGen:       flowGen,
		cycleDetector: NewCycleDetector(),
		maxSteps:      DefaultMaxSteps,
		quotas:        make(map[string]*QuotaEnforcer),
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	return e
}

// NewWithClock creates an Engine with a pre-configured clock.
// Used for replay to resume from a specific sequence number.
//
// The syncs slice is copied to prevent external mutation (CRITICAL-3 protection).
//
// Options can be passed to configure the engine (e.g., WithMaxSteps).
func NewWithClock(
	s *store.Store,
	specs []ir.ConceptSpec,
	syncs []ir.SyncRule,
	flowGen FlowTokenGenerator,
	clock *Clock,
	opts ...EngineOption,
) *Engine {
	// Copy syncs to prevent external mutation (CRITICAL-3 protection)
	var syncsCopy []ir.SyncRule
	if syncs != nil {
		syncsCopy = make([]ir.SyncRule, len(syncs))
		copy(syncsCopy, syncs)
	}

	e := &Engine{
		store:         s,
		clock:         clock,
		specs:         specs,
		syncs:         syncsCopy,
		queue:         newEventQueue(),
		flowGen:       flowGen,
		cycleDetector: NewCycleDetector(),
		maxSteps:      DefaultMaxSteps,
		quotas:        make(map[string]*QuotaEnforcer),
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Enqueue submits an event for processing by the Run loop.
// Thread-safe: may be called from any goroutine.
//
// Returns false if the engine has been stopped.
func (e *Engine) Enqueue(ev Event) bool {
	return e.queue.Enqueue(ev)
}

// NewFlow generates a new flow token for an external request.
// Thread-safe: may be called from any goroutine.
//
// Each external request (user action, webhook, scheduled job) should
// call NewFlow() once to get a correlation token. All invocations and
// completions within that request inherit the same flow token.
func (e *Engine) NewFlow() string {
	return e.flowGen.Generate()
}

// Run starts the single-writer event loop.
// Blocks until context is cancelled or Stop() is called.
//
// CRITICAL: Must be called from exactly ONE goroutine.
// All event processing, store writes, and sync rule evaluation
// happen in this goroutine for deterministic behavior.
//
// ERROR HANDLING: On event processing failure, the error is logged with full
// event context and processing continues. This "log and continue" behavior is
// intentional for determinism - retries would cause non-deterministic replay.
// Operators can use the logged event details for manual investigation/replay.
func (e *Engine) Run(ctx context.Context) error {
	slog.Info("engine starting")

	for {
		// Try non-blocking dequeue first
		event, ok := e.queue.TryDequeue()
		if ok {
			if err := e.processEvent(ctx, event); err != nil {
				// Log with full event context for manual recovery/replay
				// Design: "log and continue" preserves determinism (retries would not)
				logEventError(event, err)
			}
			continue
		}

		// No event ready - wait for signal or context cancellation
		select {
		case <-ctx.Done():
			slog.Info("engine stopping: context cancelled")
			e.queue.Close()
			return ctx.Err()

		case <-e.queue.Wait():
			// Signal received - loop back to TryDequeue
			// The signal channel closes when queue is closed,
			// which will cause this case to fire immediately
			if e.queue.Len() == 0 {
				// Queue closed and empty
				slog.Info("engine stopping: queue closed")
				return nil
			}
		}
	}
}

// Stop gracefully shuts down the engine.
// Closes the event queue, which will cause Run() to return.
func (e *Engine) Stop() {
	e.queue.Close()
}

// processEvent routes an event to the appropriate handler.
// CRITICAL: Called only from Run() goroutine - single-writer guarantee.
func (e *Engine) processEvent(ctx context.Context, event Event) error {
	switch event.Type {
	case EventTypeInvocation:
		if event.Invocation == nil {
			return fmt.Errorf("invocation event missing invocation data")
		}
		return e.processInvocation(ctx, event.Invocation)

	case EventTypeCompletion:
		if event.Completion == nil {
			return fmt.Errorf("completion event missing completion data")
		}
		return e.processCompletion(ctx, event.Completion)

	default:
		return fmt.Errorf("unknown event type: %d", event.Type)
	}
}

// processInvocation handles an invocation event.
// Writes the invocation to the store.
// CRITICAL: Called only from Run() goroutine - single-writer guarantee.
func (e *Engine) processInvocation(ctx context.Context, inv *ir.Invocation) error {
	slog.Debug("processing invocation",
		"id", inv.ID,
		"action", inv.ActionURI,
		"flow", inv.FlowToken,
		"seq", inv.Seq,
	)

	// Write invocation to store (idempotent via ON CONFLICT)
	if err := e.store.WriteInvocation(ctx, *inv); err != nil {
		return fmt.Errorf("write invocation %s: %w", inv.ID, err)
	}

	slog.Info("invocation written",
		"id", inv.ID,
		"action", inv.ActionURI,
		"flow", inv.FlowToken,
	)

	return nil
}

// processCompletion handles a completion event.
// Writes the completion to the store, then evaluates sync rules.
// CRITICAL: Called only from Run() goroutine - single-writer guarantee.
//
// QUOTA ENFORCEMENT (Story 5.4): Each completion counts against the flow's
// quota. If the quota is exceeded, sync rules are NOT evaluated and the
// flow terminates with StepsExceededError.
func (e *Engine) processCompletion(ctx context.Context, comp *ir.Completion) error {
	slog.Debug("processing completion",
		"id", comp.ID,
		"invocation_id", comp.InvocationID,
		"output_case", comp.OutputCase,
		"seq", comp.Seq,
	)

	// Write completion to store (idempotent via ON CONFLICT)
	if err := e.store.WriteCompletion(ctx, *comp); err != nil {
		return fmt.Errorf("write completion %s: %w", comp.ID, err)
	}

	slog.Info("completion written",
		"id", comp.ID,
		"invocation_id", comp.InvocationID,
		"output_case", comp.OutputCase,
	)

	// Get the flow token from the originating invocation
	inv, err := e.store.ReadInvocation(ctx, comp.InvocationID)
	if err != nil {
		return fmt.Errorf("read invocation for flow token: %w", err)
	}
	flowToken := inv.FlowToken

	// QUOTA ENFORCEMENT (Story 5.4): Check quota before evaluating sync rules
	// Get or create quota enforcer for this flow
	quota, exists := e.quotas[flowToken]
	if !exists {
		quota = NewQuotaEnforcer(e.maxSteps)
		e.quotas[flowToken] = quota
	}

	// Check quota - if exceeded, flow terminates
	if err := quota.Check(flowToken); err != nil {
		slog.Error("max steps quota exceeded",
			"flow_token", flowToken,
			"completion_id", comp.ID,
			"steps", quota.Current(),
			"limit", e.maxSteps,
			"event", "quota_exceeded",
		)
		return fmt.Errorf("quota enforcement failed: %w", err)
	}

	// Evaluate sync rules (CRITICAL-3: declaration order)
	if err := e.evaluateSyncs(ctx, comp); err != nil {
		return fmt.Errorf("evaluate syncs for completion %s: %w", comp.ID, err)
	}

	return nil
}

// evaluateSyncs evaluates all registered sync rules against a completion.
//
// Sync rules are checked in declaration order (CRITICAL-3). For each
// matching sync, bindings are extracted and invocations are generated.
//
// To check the when-clause, we need the original invocation (for ActionURI).
// This is retrieved from the store using comp.InvocationID.
//
// Flow token propagation (Story 3.6): The flow token is inherited from the
// triggering invocation, never generated mid-flow.
//
// TODO (Story 3.7): Implement flow-scoped where-clause execution
func (e *Engine) evaluateSyncs(ctx context.Context, comp *ir.Completion) error {
	// Lookup the invocation (needed for action URI matching and flow token)
	inv, err := e.store.ReadInvocation(ctx, comp.InvocationID)
	if err != nil {
		return fmt.Errorf("read invocation %s: %w", comp.InvocationID, err)
	}

	// Flow token is inherited from the invocation (Story 3.6: CP-7)
	flowToken := inv.FlowToken

	// Iterate syncs in declaration order (deterministic)
	for _, sync := range e.syncs {
		// Check if this sync matches the completion
		if matchWhen(sync.When, &inv, comp) {
			slog.Debug("sync rule matched",
				"sync_id", sync.ID,
				"completion_id", comp.ID,
				"action", inv.ActionURI,
				"flow_token", flowToken,
			)

			// Extract bindings from the when-clause
			bindings, err := extractBindings(sync.When, comp)
			if err != nil {
				slog.Warn("binding extraction failed",
					"sync_id", sync.ID,
					"completion_id", comp.ID,
					"error", err,
				)
				// Continue to next sync - binding failure shouldn't stop evaluation
				continue
			}

			slog.Debug("bindings extracted",
				"sync_id", sync.ID,
				"binding_count", len(bindings),
			)

			// Fire the sync rule with inherited flow token (Story 3.6)
			// TODO (Story 3.7): Execute where-clause for multi-binding scenarios
			if err := e.fireSyncRule(ctx, sync, comp, flowToken, bindings); err != nil {
				slog.Error("sync rule firing failed",
					"sync_id", sync.ID,
					"completion_id", comp.ID,
					"error", err,
				)
				// Continue to next sync - individual sync failure shouldn't stop evaluation
				continue
			}
		}
	}

	return nil
}

// fireSyncRule executes a sync rule for a specific binding set.
// This is called by evaluateSyncs for each matching sync/binding pair.
//
// The flow token is inherited from the triggering invocation (CP-7).
// Flow token is NEVER generated mid-flow - it's a parameter, not computed.
//
// CRASH ATOMICITY (CP-1): Uses WriteSyncFiringAtomic to write the firing,
// invocation, and provenance edge in a single transaction. This prevents
// orphaned invocations on crash recovery.
func (e *Engine) fireSyncRule(ctx context.Context, sync ir.SyncRule, comp *ir.Completion, flowToken string, bindings ir.IRObject) error {
	// Compute binding hash for idempotency check (CP-1)
	bindingHash, err := ir.BindingHash(bindings)
	if err != nil {
		return fmt.Errorf("compute binding hash: %w", err)
	}

	// Generate invocation with INHERITED flow token (Story 3.6)
	// We generate this before the atomic write so we have the full invocation ready
	inv, err := e.generateInvocation(flowToken, sync.Then, bindings)
	if err != nil {
		return fmt.Errorf("generate invocation: %w", err)
	}

	// Prepare firing record
	firing := ir.SyncFiring{
		CompletionID: comp.ID,
		SyncID:       sync.ID,
		BindingHash:  bindingHash,
		Seq:          e.clock.Next(),
	}

	// ATOMIC: Write firing + invocation + provenance in single transaction
	// This ensures crash atomicity - either all three are written or none
	_, inserted, err := e.store.WriteSyncFiringAtomic(ctx, firing, inv)
	if err != nil {
		return fmt.Errorf("atomic sync firing: %w", err)
	}

	if !inserted {
		slog.Debug("sync already fired, skipping (idempotent)",
			"sync_id", sync.ID,
			"completion_id", comp.ID,
			"binding_hash", bindingHash,
		)
		return nil
	}

	slog.Info("sync fired",
		"sync_id", sync.ID,
		"completion_id", comp.ID,
		"invocation_id", inv.ID,
		"flow_token", flowToken,
		"action_uri", inv.ActionURI,
		"seq", inv.Seq,
	)

	return nil
}

// generateInvocation creates a new invocation from a then-clause.
// The flow token is INHERITED from the triggering completion, never generated.
//
// Parameters:
//   - flowToken: Flow token from the invocation that triggered this sync
//   - then: Then-clause from sync rule (action + arg templates)
//   - bindings: Variable bindings from when-clause (and where-clause in future)
//
// Returns:
//   - Invocation with inherited flow token and computed content-addressed ID
//   - Error if arg resolution fails
//
// CRITICAL: Flow token is a PARAMETER, not generated. This ensures flow
// token chain remains unbroken from root to leaf (CP-7).
func (e *Engine) generateInvocation(flowToken string, then ir.ThenClause, bindings ir.IRObject) (ir.Invocation, error) {
	// Validate flow token - required for propagation chain integrity
	if flowToken == "" {
		return ir.Invocation{}, fmt.Errorf("flow token is required")
	}

	// Resolve args from then-clause templates and bindings
	args, err := e.resolveArgs(then.Args, bindings)
	if err != nil {
		return ir.Invocation{}, fmt.Errorf("resolve args for action %s: %w", then.ActionRef, err)
	}

	// Get sequence number
	seq := e.clock.Next()

	// Compute content-addressed ID
	id, err := ir.InvocationID(flowToken, then.ActionRef, args, seq)
	if err != nil {
		return ir.Invocation{}, fmt.Errorf("compute invocation ID: %w", err)
	}

	// Build invocation with inherited flow token
	inv := ir.Invocation{
		ID:              id,
		FlowToken:       flowToken, // INHERITED from triggering invocation
		ActionURI:       ir.ActionRef(then.ActionRef),
		Args:            args,
		Seq:             seq,
		SecurityContext: e.currentSecurityContext(),
		SpecHash:        e.specHash,
		EngineVersion:   ir.EngineVersion,
		IRVersion:       ir.IRVersion,
	}

	return inv, nil
}

// resolveArgs substitutes binding variables into then-clause arg templates.
//
// The then.Args map has string keys (arg names) and string values (expressions).
// String values starting with "${bound." are binding references that get substituted.
// Other values are treated as literal strings.
//
// Example:
//
//	then.Args = { "product_id": "${bound.product}", "status": "pending" }
//	bindings  = { "product": IRString("widget") }
//	result    = { "product_id": IRString("widget"), "status": IRString("pending") }
func (e *Engine) resolveArgs(argTemplates map[string]string, bindings ir.IRObject) (ir.IRObject, error) {
	resolved := make(ir.IRObject, len(argTemplates))

	for key, template := range argTemplates {
		val, err := e.substituteBinding(template, bindings)
		if err != nil {
			return nil, fmt.Errorf("substitute binding for key %q: %w", key, err)
		}
		resolved[key] = val
	}

	return resolved, nil
}

// substituteBinding replaces a binding reference with its actual value.
// Binding references use the format "${bound.varname}".
// Non-reference strings are returned as IRString literals.
//
// Examples:
//
//	"${bound.product}" with bindings{"product": IRString("widget")} → IRString("widget")
//	"literal value" → IRString("literal value")
func (e *Engine) substituteBinding(template string, bindings ir.IRObject) (ir.IRValue, error) {
	const prefix = "${bound."
	const suffix = "}"

	// Check if this is a binding reference
	if len(template) > len(prefix)+len(suffix) &&
		template[:len(prefix)] == prefix &&
		template[len(template)-len(suffix):] == suffix {
		// Extract binding name
		bindingName := template[len(prefix) : len(template)-len(suffix)]

		// Look up in bindings
		val, ok := bindings[bindingName]
		if !ok {
			return nil, fmt.Errorf("binding %q not found", bindingName)
		}
		return val, nil
	}

	// Not a binding reference - return as literal string
	return ir.IRString(template), nil
}

// currentSecurityContext returns the security context for generated invocations.
// Currently returns a placeholder - full implementation in Story 4.x (authz).
//
// TODO: Inherit from triggering invocation or use engine-level context
func (e *Engine) currentSecurityContext() ir.SecurityContext {
	return ir.SecurityContext{
		TenantID:    "default",
		UserID:      "engine",
		Permissions: []string{},
	}
}

// executeWhereClause executes a where-clause query with scope filtering.
// Returns a slice of binding sets (one per matching record).
//
// Flow-scoping modes (Story 3.7):
//   - "flow" (default): Add flow_token filter to match only same flow
//   - "global": No flow_token filter (match all flows)
//   - "keyed": Filter by specified key field value
//
// STUB: Full implementation requires QueryIR from Epic 4. Currently returns
// the when-bindings as a single binding set (no actual query execution).
//
// TODO (Epic 4): Implement full query execution with scope filters
func (e *Engine) executeWhereClause(
	ctx context.Context,
	sync ir.SyncRule,
	flowToken string,
	whenBindings ir.IRObject,
) ([]ir.IRObject, error) {
	// If no where-clause, return single binding set (when-bindings only)
	if sync.Where == nil {
		return []ir.IRObject{whenBindings}, nil
	}

	// Normalize scope mode (default to "flow")
	scope := NormalizeScope(sync.Scope)

	// Validate scope mode
	if err := ValidateScopeMode(scope.Mode); err != nil {
		return nil, fmt.Errorf("invalid scope: %w", err)
	}

	// Validate keyed scope has required key
	if ScopeMode(scope.Mode) == ScopeModeKeyed {
		if scope.Key == "" {
			return nil, fmt.Errorf("keyed scope requires non-empty key field")
		}
		// Validate that key exists in when-bindings
		_, err := extractKeyValue(whenBindings, scope.Key)
		if err != nil {
			return nil, fmt.Errorf("extract key value for keyed scope: %w", err)
		}
	}

	// TODO (Epic 4): Execute actual query with scope filter
	// For now, return when-bindings as single binding set
	// This allows sync rules without where-clauses to work correctly
	slog.Debug("executeWhereClause stub: returning when-bindings only",
		"sync_id", sync.ID,
		"scope_mode", scope.Mode,
		"flow_token", flowToken,
		"has_where", sync.Where != nil,
	)

	return []ir.IRObject{whenBindings}, nil
}

// RegisterSyncs registers sync rules with the engine in declaration order.
//
// The syncs slice order determines evaluation priority. Sync rules are
// evaluated in the exact order provided, which must match the declaration
// order from the CUE compiler.
//
// This function validates:
//   - All sync IDs are unique
//   - All when-clause event types are supported ("completed" only for now)
//
// Passing nil or an empty slice is valid and clears any previously
// registered sync rules.
//
// CRITICAL: This order is deterministic and preserved across engine restarts.
// The same input order guarantees the same evaluation order.
func (e *Engine) RegisterSyncs(syncs []ir.SyncRule) error {
	if syncs == nil {
		e.syncs = nil
		return nil
	}

	// Validate uniqueness of sync IDs and supported event types
	seen := make(map[string]bool, len(syncs))
	for _, sync := range syncs {
		if seen[sync.ID] {
			return fmt.Errorf("duplicate sync ID: %s", sync.ID)
		}
		seen[sync.ID] = true

		// Validate event type is supported
		// Currently only "completed" is implemented; "invoked" is planned for future
		if sync.When.EventType != "" && sync.When.EventType != "completed" {
			return fmt.Errorf("sync %s: unsupported event type %q (only \"completed\" is currently supported)", sync.ID, sync.When.EventType)
		}
	}

	// Store syncs in declaration order
	// Make a copy to prevent external mutation
	e.syncs = make([]ir.SyncRule, len(syncs))
	copy(e.syncs, syncs)

	return nil
}

// Syncs returns the registered sync rules in declaration order.
// Used for testing and introspection.
func (e *Engine) Syncs() []ir.SyncRule {
	return e.syncs
}

// Clock returns the engine's logical clock.
// Used by external code to stamp events before enqueuing.
func (e *Engine) Clock() *Clock {
	return e.clock
}

// QueueLen returns the current number of pending events.
// Useful for monitoring and testing.
func (e *Engine) QueueLen() int {
	return e.queue.Len()
}

// ClearFlowCycleHistory removes cycle detection history for a flow.
// Should be called when a flow completes (success or error) to prevent memory leaks.
//
// Thread-safe: delegates to CycleDetector which uses internal locking.
func (e *Engine) ClearFlowCycleHistory(flowToken string) {
	e.cycleDetector.Clear(flowToken)
}

// CycleDetectorForTesting returns the cycle detector for testing purposes.
// Not intended for production use.
func (e *Engine) CycleDetectorForTesting() *CycleDetector {
	return e.cycleDetector
}

// CleanupFlow removes quota enforcer and cycle history for a completed flow.
// Should be called when a flow reaches terminal state to prevent memory leaks.
//
// This removes:
//   - Quota enforcer from quotas map
//   - Cycle detection history from cycleDetector
func (e *Engine) CleanupFlow(flowToken string) {
	delete(e.quotas, flowToken)
	e.cycleDetector.Clear(flowToken)
}

// MaxSteps returns the configured maximum steps per flow.
// Used for testing and diagnostics.
func (e *Engine) MaxSteps() int {
	return e.maxSteps
}

// QuotaFor returns or creates the quota enforcer for a specific flow.
// Creates a new enforcer if one doesn't exist.
// Used for testing and diagnostics.
func (e *Engine) QuotaFor(flowToken string) *QuotaEnforcer {
	if q, ok := e.quotas[flowToken]; ok {
		return q
	}
	q := NewQuotaEnforcer(e.maxSteps)
	e.quotas[flowToken] = q
	return q
}

// QuotaCount returns the number of active quota enforcers.
// Used for testing to verify cleanup.
func (e *Engine) QuotaCount() int {
	return len(e.quotas)
}

// logEventError logs an event processing failure with full context.
// This enables manual investigation and potential replay of failed events.
func logEventError(event Event, err error) {
	switch event.Type {
	case EventTypeInvocation:
		if event.Invocation != nil {
			slog.Error("invocation processing failed",
				"error", err,
				"invocation_id", event.Invocation.ID,
				"flow_token", event.Invocation.FlowToken,
				"action_uri", event.Invocation.ActionURI,
				"seq", event.Invocation.Seq,
			)
		} else {
			slog.Error("invocation processing failed",
				"error", err,
				"event_type", "invocation",
				"note", "invocation data was nil",
			)
		}

	case EventTypeCompletion:
		if event.Completion != nil {
			slog.Error("completion processing failed",
				"error", err,
				"completion_id", event.Completion.ID,
				"invocation_id", event.Completion.InvocationID,
				"output_case", event.Completion.OutputCase,
				"seq", event.Completion.Seq,
			)
		} else {
			slog.Error("completion processing failed",
				"error", err,
				"event_type", "completion",
				"note", "completion data was nil",
			)
		}

	default:
		slog.Error("event processing failed",
			"error", err,
			"event_type", event.Type,
		)
	}
}
