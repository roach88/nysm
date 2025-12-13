Major Corrections from Reading the Actual Paper:

  What We Got Wrong/Missed:

  | Our Assumption              | Paper's Actual Approach
                                                                              |
  |-----------------------------|--------------------------------------------------------------------
  ----------------------------------------------------------------------------|
  | Generic "events"            | Action invocations/completions with named args and typed outputs
                                                                              |
  | Simple frontmatter          | Full concept spec language: purpose, relational state schema,
  action signatures with multiple cases (including errors), operational principles |
  | Vague "when X do Y"         | 3-clause DSL: when (action completions) → where (state queries +
  bindings) → then (invocations)                                                |
  | Implicit scoping            | Flow tokens - explicit scoping mechanism that prevents accidental
  joins across concurrent requests                                             |
  | Transaction-based integrity | Explicitly abandons transactions - uses rule scheme + flow scoping
  + error-matching instead                                                    |
  | Capability injection        | Stricter: No getters! Reads via query (SPARQL/GraphQL), writes via
  actions only                                                                |
  | Logging for debuggability   | Structural provenance - engine records sync edges for idempotency +
   restart/replay                                                             |

  Critical Insight We Totally Missed:

  Operational Principles - Each concept spec includes an archetypal scenario that:
  - Anchors human understanding of what the concept does
  - Becomes an executable contract test
  - Is THE documentation that can't rot because it's validated

  ---
  Paper-Faithful Minimal Viable Architecture:

  Artifacts:

  concepts/
    Cart.concept          # Full spec: purpose, state, actions, operational principle
    Inventory.concept

  sync/
    cart-inventory.sync   # when {...} where {...} then {...}

  bootstrap/
    Web.concept           # request/respond actions (replaces routes!)

  Runtime Model (the actual "WYSI" part):

  1. Action Records: Invocation(flow, action, named-args) and Completion(flow, action, named-args, 
  named-results)
  2. Flow Tokens: Every record has a flow ID; syncs only match same-flow records
  3. Where Clause Queries: Binding → set-of-bindings transform (like joins without loops)
  4. Provenance Edges: (completion) -[sync-id]-> (invocation) for idempotent firing + replay
  5. Naming/Versioning: Fully-qualified URIs + version markers with every record

  ---
  Expert-Level Build Path:

  1. Define Core IR + Semantics First (before any SDK)
    - IR nodes: ConceptSpec, ActionSig, StateRel, SyncRule, Flow, Invocation, Completion,
  ProvenanceEdge
    - Formalize rule evaluation semantics
  2. Implement Durable Engine as a Database Problem
    - Store invocations/completions + provenance edges with strong query support
    - Implement idempotency via "sync edges" check for crash-restart replay
  3. Pick Query Substrate for where Clause
    - Paper uses RDF + SPARQL (graph semantics)
    - Alternative: Compile DSL to SQL over relational schemas
  4. Make Naming/Versioning Non-Optional
    - URI scheme for concepts/actions/args/syncs
    - "Causal documentation" - persist app version with every record
    - Build diff tooling at DSL level
  5. Build Concept Codegen + Conformance Harness
    - Generate typed stubs from concept specs
    - Enforce named args, typed output cases
    - Operational principle = executable contract test
  6. Web/Bootstrap as Ordinary Concept
    - Web/request completion ingestion + Web/respond emission
    - Auth/authz as synchronizations gating Web/request → domain invocations
  7. Observability as First-Class Product
    - Deterministic trace reconstruction per flow
    - "Why did this happen?" answered by provenance edges + sync IDs
