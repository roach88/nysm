---
story_id: "7.8"
epic: "Epic 7: CLI & Demo Application"
title: "Demo Concept Specs"
status: "done"
created: "2025-12-12"
dependencies:
  - "7.7"
points: "3"
---

# Story 7.8: Demo Concept Specs

## Story Statement

**As a** developer learning NYSM,
**I want** canonical demo specs for Cart, Inventory, and Web concepts,
**So that** I have a reference implementation to learn from.

## Context

This story delivers the canonical demo specifications that serve as:
1. Reference implementation for NYSM's WYSIWYG pattern
2. Test fixtures for the conformance harness
3. Documentation by example
4. Validation that the framework implements the paper faithfully

These specs must be self-documenting and demonstrate all key NYSM features: typed action outputs with error variants, relational state, operational principles, and coordination via sync rules.

## Acceptance Criteria

### AC-1: Cart Concept Spec

**Given** the specs directory
**When** I read `specs/cart.concept.cue`
**Then** I find a complete Cart concept with:
- Purpose statement
- CartItem state schema
- `addItem`, `removeItem`, `checkout` actions
- Typed outputs with success and error cases
- Operational principles

**Example CUE:**
```cue
concept Cart {
    purpose: "Manages shopping cart state for a user session"

    state CartItem {
        item_id: string
        quantity: int
    }

    action addItem {
        args: {
            item_id: string
            quantity: int
        }
        outputs: [{
            case: "Success"
            fields: { item_id: string, new_quantity: int }
        }, {
            case: "InvalidQuantity"
            fields: { message: string }
        }]
    }

    action removeItem {
        args: { item_id: string }
        outputs: [{
            case: "Success"
            fields: {}
        }, {
            case: "ItemNotFound"
            fields: { item_id: string }
        }]
    }

    action checkout {
        args: {}
        outputs: [{
            case: "Success"
            fields: { cart_id: string }
        }, {
            case: "EmptyCart"
            fields: {}
        }]
    }

    operational_principle: """
        Adding an item that exists increases quantity.
        Removing an item decreases quantity or removes entry.
        Checkout with empty cart returns EmptyCart error.
        """
}
```

### AC-2: Inventory Concept Spec

**Given** the specs directory
**When** I read `specs/inventory.concept.cue`
**Then** I find a complete Inventory concept with:
- Purpose statement
- InventoryItem state schema
- `reserve`, `release` actions
- Error variants for insufficient stock
- Operational principles

**Example CUE:**
```cue
concept Inventory {
    purpose: "Manages inventory stock levels with reservation semantics"

    state InventoryItem {
        item_id: string
        quantity: int
        reserved: int
    }

    action reserve {
        args: {
            item_id: string
            quantity: int
        }
        outputs: [{
            case: "Success"
            fields: { reservation_id: string }
        }, {
            case: "InsufficientStock"
            fields: {
                item_id: string
                available: int
                requested: int
            }
        }, {
            case: "InvalidQuantity"
            fields: { message: string }
        }]
    }

    action release {
        args: {
            reservation_id: string
        }
        outputs: [{
            case: "Success"
            fields: {}
        }, {
            case: "ReservationNotFound"
            fields: { reservation_id: string }
        }]
    }

    operational_principle: """
        Reserve reduces available stock and tracks reservation.
        Reserve fails if insufficient stock available.
        Release restores reserved stock to available.
        """
}
```

### AC-3: Web Concept Spec

**Given** the specs directory
**When** I read `specs/web.concept.cue`
**Then** I find a complete Web concept with:
- Purpose statement
- Request/Response state schema
- `request`, `respond` actions
- HTTP-like semantics
- Operational principles

**Example CUE:**
```cue
concept Web {
    purpose: "HTTP-like request/response coordination for external interactions"

    state Request {
        request_id: string
        path: string
        method: string
        body: string
        flow_token: string
    }

    state Response {
        request_id: string
        status: int
        body: string
    }

    action request {
        args: {
            path: string
            method: string
            body: string
        }
        outputs: [{
            case: "Success"
            fields: { request_id: string }
        }]
    }

    action respond {
        args: {
            request_id: string
            status: int
            body: string
        }
        outputs: [{
            case: "Success"
            fields: {}
        }, {
            case: "RequestNotFound"
            fields: { request_id: string }
        }]
    }

    operational_principle: """
        Request creates pending request record.
        Respond completes request with status and body.
        Request can only be responded to once.
        """
}
```

### AC-4: Specs Compile and Validate

**Given** the demo specs
**When** I run `nysm compile ./specs`
**Then** compilation succeeds with no errors
**And** output shows:
```
✓ Compiled 3 concepts

Concepts:
  Cart: 3 actions, 1 operational principle
  Inventory: 2 actions, 1 operational principle
  Web: 2 actions, 1 operational principle
```

### AC-5: Specs Are Self-Documenting

**Given** a developer reading the specs
**When** they examine the CUE files
**Then** they can understand:
- What each concept does (from purpose)
- What state it maintains (from state schema)
- What operations are available (from actions)
- How success and errors work (from output cases)
- Expected behavior (from operational principles)

**Without** needing external documentation.

## Quick Reference

### Key Patterns Demonstrated

| Pattern | Example | Location |
|---------|---------|----------|
| **Typed Error Variants** | `InsufficientStock`, `ItemNotFound` | All actions |
| **Named Arguments** | `item_id: string, quantity: int` | All actions |
| **Operational Principles** | Archetypal scenarios as strings | All concepts |
| **Relational State** | `CartItem`, `InventoryItem` | State schemas |
| **Self-Documentation** | Purpose + schemas + principles | All specs |

### File Locations

```
specs/
├── cart.concept.cue         # Cart concept with addItem, removeItem, checkout
├── inventory.concept.cue    # Inventory concept with reserve, release
└── web.concept.cue          # Web concept with request, respond
```

## Tasks/Subtasks

### Task 1: Create Cart Concept Spec
- [ ] Define purpose statement
- [ ] Define CartItem state schema with item_id and quantity
- [ ] Define addItem action with Success and InvalidQuantity cases
- [ ] Define removeItem action with Success and ItemNotFound cases
- [ ] Define checkout action with Success and EmptyCart cases
- [ ] Write operational principle covering add, remove, checkout scenarios
- [ ] Validate spec compiles correctly

### Task 2: Create Inventory Concept Spec
- [ ] Define purpose statement
- [ ] Define InventoryItem state schema with item_id, quantity, reserved
- [ ] Define reserve action with Success, InsufficientStock, InvalidQuantity cases
- [ ] Define release action with Success and ReservationNotFound cases
- [ ] Write operational principle covering reserve/release semantics
- [ ] Validate spec compiles correctly

### Task 3: Create Web Concept Spec
- [ ] Define purpose statement
- [ ] Define Request state schema with request_id, path, method, body, flow_token
- [ ] Define Response state schema with request_id, status, body
- [ ] Define request action with Success case
- [ ] Define respond action with Success and RequestNotFound cases
- [ ] Write operational principle covering request/respond flow
- [ ] Validate spec compiles correctly

### Task 4: Integration Testing
- [ ] Run nysm compile on all specs
- [ ] Verify compilation output matches expected format
- [ ] Confirm no validation errors
- [ ] Test specs load correctly in harness

## Dev Notes

### Implementation Details

#### specs/cart.concept.cue

**State Management:**
- `CartItem` uses composite key of (flow_token, item_id)
- Quantity must be positive integer
- Empty cart defined as zero items

**Action Behaviors:**
- `addItem`: If item exists, increment quantity; else create new CartItem
- `removeItem`: Delete CartItem entry (quantity tracking not needed)
- `checkout`: Validates cart non-empty, generates cart_id from flow_token

**Error Cases:**
- `InvalidQuantity`: quantity <= 0
- `ItemNotFound`: removeItem on non-existent item
- `EmptyCart`: checkout with no cart items

**Operational Principle:**
```
GIVEN: Empty cart
WHEN: addItem(widget, 3)
THEN: CartItem(widget, 3) exists

GIVEN: CartItem(widget, 3)
WHEN: addItem(widget, 2)
THEN: CartItem(widget, 5) exists

GIVEN: CartItem(widget, 3)
WHEN: removeItem(widget)
THEN: CartItem(widget) does not exist

GIVEN: Empty cart
WHEN: checkout()
THEN: EmptyCart error
```

#### specs/inventory.concept.cue

**State Management:**
- `InventoryItem` tracks both available and reserved quantities
- `available = quantity - reserved`
- Reservations tracked in separate table (not shown in state schema)

**Action Behaviors:**
- `reserve`: Check available >= requested, create reservation, increment reserved
- `release`: Find reservation, decrement reserved, delete reservation record

**Error Cases:**
- `InsufficientStock`: available < requested (returns available count)
- `InvalidQuantity`: quantity <= 0
- `ReservationNotFound`: release with invalid reservation_id

**Operational Principle:**
```
GIVEN: InventoryItem(widget, quantity=10, reserved=0)
WHEN: reserve(widget, 3)
THEN: InventoryItem(widget, quantity=10, reserved=3)
AND: reservation_id returned

GIVEN: InventoryItem(widget, quantity=10, reserved=3)
WHEN: reserve(widget, 8)
THEN: InsufficientStock(available=7, requested=8)

GIVEN: reservation_id for 3 widgets
WHEN: release(reservation_id)
THEN: reserved decrements by 3
```

#### specs/web.concept.cue

**State Management:**
- `Request` persists until responded
- `Response` linked to Request via request_id
- flow_token propagates through request/response cycle

**Action Behaviors:**
- `request`: Create Request record, generate request_id from flow_token
- `respond`: Create Response record, mark Request as completed

**Error Cases:**
- `RequestNotFound`: respond to non-existent request

**Operational Principle:**
```
GIVEN: External HTTP request arrives
WHEN: request(path=/checkout, method=POST, body={...})
THEN: Request record created with request_id

GIVEN: Request(request_id=R1) pending
WHEN: respond(request_id=R1, status=200, body={...})
THEN: Response(request_id=R1) created
AND: Request marked complete

GIVEN: No Request with request_id=R2
WHEN: respond(request_id=R2, ...)
THEN: RequestNotFound error
```

### CUE Validation Rules

Each concept spec must pass:

1. **Required Fields:**
   - `purpose` non-empty string
   - At least one `state` definition
   - At least one `action` definition
   - At least one `operational_principle`

2. **State Schema:**
   - Field names: snake_case
   - Field types: string, int, bool only (no floats)

3. **Action Signatures:**
   - `args`: object with typed fields
   - `outputs`: array with at least one OutputCase
   - Each OutputCase has `case` (string) and `fields` (object)

4. **Naming Conventions:**
   - Concept names: PascalCase
   - Action names: camelCase
   - State names: PascalCase
   - Field names: snake_case

### Compilation Pipeline

```
[CUE Specs] → [CUE SDK Parser] → [Validation] → [IR Generation] → [Canonical JSON]
                                        ↓
                                  Error Messages
                                  (file:line)
```

**Compiler Output:**
- Validates against CUE constraints
- Generates `ir.ConceptSpec` for each concept
- Serializes to canonical JSON (RFC 8785)
- Reports validation errors with file:line references

### Test Scenarios

The specs will be tested via:

1. **Compilation Tests:**
   - Valid specs compile without errors
   - Invalid specs produce clear error messages

2. **Conformance Tests:**
   - Operational principles become test scenarios
   - Harness validates behavior matches principles

3. **Golden Snapshots:**
   - Compiled IR stored as golden files
   - Changes trigger snapshot updates

## Test Examples

### Test 1: Cart Add Item Success

```yaml
# testdata/scenarios/cart_add_item.yaml
name: cart_add_item
description: "Adding item to cart creates or updates CartItem"

specs:
  - specs/cart.concept.cue

flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 3 }
    expect:
      case: Success
      result: { item_id: "widget", new_quantity: 3 }

assertions:
  - type: final_state
    table: cart_items
    where: { item_id: "widget" }
    expect: { quantity: 3 }
```

### Test 2: Inventory Insufficient Stock

```yaml
# testdata/scenarios/inventory_insufficient_stock.yaml
name: inventory_insufficient_stock
description: "Reserve fails when insufficient stock available"

specs:
  - specs/inventory.concept.cue

setup:
  - action: Inventory.setStock
    args: { item_id: "widget", quantity: 5 }

flow:
  - invoke: Inventory.reserve
    args: { item_id: "widget", quantity: 10 }
    expect:
      case: InsufficientStock
      result: {
        item_id: "widget",
        available: 5,
        requested: 10
      }

assertions:
  - type: final_state
    table: inventory_items
    where: { item_id: "widget" }
    expect: { quantity: 5, reserved: 0 }
```

### Test 3: Web Request/Response Flow

```yaml
# testdata/scenarios/web_request_respond.yaml
name: web_request_respond
description: "Request creates record, respond completes it"

specs:
  - specs/web.concept.cue

flow:
  - invoke: Web.request
    args: { path: "/checkout", method: "POST", body: "{}" }
    expect:
      case: Success
    bind:
      request_id: result.request_id

  - invoke: Web.respond
    args: {
      request_id: bound.request_id,
      status: 200,
      body: "{\"success\":true}"
    }
    expect:
      case: Success

assertions:
  - type: final_state
    table: requests
    where: { request_id: bound.request_id }
    expect: { completed: true }

  - type: final_state
    table: responses
    where: { request_id: bound.request_id }
    expect: { status: 200 }
```

## File List

### Created Files

```
specs/cart.concept.cue
specs/inventory.concept.cue
specs/web.concept.cue
```

### Modified Files

None (new spec files only)

### Test Files

```
testdata/scenarios/cart_add_item.yaml
testdata/scenarios/cart_remove_item.yaml
testdata/scenarios/cart_checkout_empty.yaml
testdata/scenarios/inventory_reserve_success.yaml
testdata/scenarios/inventory_insufficient_stock.yaml
testdata/scenarios/inventory_release.yaml
testdata/scenarios/web_request_respond.yaml
```

### Golden Files

```
testdata/golden/compiler/cart_concept.golden
testdata/golden/compiler/inventory_concept.golden
testdata/golden/compiler/web_concept.golden
```

## Relationship to Other Stories

### Depends On
- **Story 7.7: Trace Command** - CLI infrastructure ready for testing specs
- **Epic 1 (Foundation)** - IR types and compilation pipeline
- **Epic 6 (Harness)** - Test runner for operational principles

### Enables
- **Story 7.9: Demo Sync Rules** - Specs provide concepts to coordinate
- **Story 7.10: Demo Scenarios** - Specs used in golden trace tests
- **Conformance Testing** - Specs become test fixtures

### Related
- **Story 1.6: CUE Concept Spec Parser** - Compiler implementation
- **Story 6.5: Operational Principle Validation** - Principles as tests
- **PRD Appendix A** - Canonical demo scenarios

## Story Completion Checklist

### Definition of Ready
- [x] Story is sized and estimated
- [x] Acceptance criteria are clear and testable
- [x] Dependencies identified and resolved
- [x] Dev notes provide sufficient implementation detail

### Definition of Done
- [ ] All three concept specs created and committed
- [ ] Specs compile without errors via `nysm compile`
- [ ] All actions have typed outputs with error variants
- [ ] Operational principles documented for each concept
- [ ] Test scenarios created for key behaviors
- [ ] Golden files generated and committed
- [ ] Code reviewed against architecture patterns
- [ ] No linter warnings or errors
- [ ] All tests pass (unit + integration)

### Validation Steps
- [ ] Run `nysm compile ./specs` - succeeds with expected output
- [ ] Run `nysm validate ./specs` - no errors
- [ ] Read each spec - understandable without external docs
- [ ] Run `nysm test ./specs ./testdata/scenarios` - all scenarios pass
- [ ] Check golden files - canonical JSON output matches expected

## References

### PRD References
- **Section 4.1:** Concept Specs definition
- **Section 5 (FR-1):** Concept Specification System requirements
- **Appendix A:** Canonical demo scenarios

### Architecture References
- **Section: Technology Stack** - CUE SDK v0.15.1 integration
- **Section: Core Architectural Decisions** - Typed action outputs (FR-1.4)
- **Section: Complete Project Directory Structure** - `specs/` location
- **Critical Pattern CP-5:** Constrained IRValue types (no floats)

### Paper References
- **Section 3: Concept Specs** - Purpose, state, actions, operational principles
- **Section 4: Operational Principles** - Archetypal scenarios as tests
- **Figure 2:** Cart/Inventory coordination example

### Related Stories
- **Story 1.3:** Typed Action Outputs with Error Variants
- **Story 1.6:** CUE Concept Spec Parser
- **Story 6.5:** Operational Principle Validation
- **Story 7.9:** Demo Sync Rules (uses these concepts)

## Dev Agent Record

### Session Log Template

```markdown
## Session: [Date]

### Tasks Completed
- [ ] Task 1: Create Cart Concept Spec
- [ ] Task 2: Create Inventory Concept Spec
- [ ] Task 3: Create Web Concept Spec
- [ ] Task 4: Integration Testing

### Decisions Made
- [Record any implementation decisions]

### Issues Encountered
- [Any blockers or challenges]

### Next Steps
- [What needs to happen next]
```

### Implementation Notes

**Order of Implementation:**
1. Start with Cart (simplest, well-understood)
2. Then Inventory (adds reservation complexity)
3. Finally Web (demonstrates request/response pattern)
4. Test each individually before integration

**Common Pitfalls:**
- Forgetting to include all error cases
- Using floats in state schema (forbidden per CP-5)
- Missing operational principles
- Non-descriptive purpose statements

**Quality Checks:**
- Each action has at least one success case
- Error cases have descriptive field names
- Operational principles cover happy and error paths
- Purpose statements explain "what" and "why"

---

**Story Status:** done
**Next Story:** 7.9 - Demo Sync Rules
