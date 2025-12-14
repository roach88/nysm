// Cart-Inventory Sync Rules
//
// Demonstrates NYSM's 3-clause sync pattern for coordinating Cart and Inventory concepts.
//
// PATTERN OVERVIEW:
// =================
// Each sync rule follows the when → where → then structure:
//   when:  Triggers on action completion (with optional output case matching)
//   where: Queries state tables to produce binding sets (optional)
//   then:  Generates invocations using bound variables
//
// KEY PATTERNS DEMONSTRATED:
// ==========================
// 1. CRITICAL-1 (Multi-binding): One completion → many invocations
//    - cart-inventory-reserve shows where clause returning multiple bindings
//    - Each binding produces a separate sync firing and invocation
//
// 2. MEDIUM-2 (Error matching): Match specific error variants in when clause
//    - handle-insufficient-stock matches InsufficientStock error case
//    - Error fields extracted into bindings for propagation
//
// 3. Flow scoping: All syncs use scope: "flow" (default)
//    - Only matches events within the same flow token
//    - Ensures isolation between concurrent flows
package specs

// Sync Rule: cart-inventory-reserve
//
// PATTERN: Multi-binding (CRITICAL-1)
// When cart checkout succeeds, reserve inventory for each cart item.
//
// The where clause queries CartItem state and returns multiple bindings
// (one per item in the cart). Each binding produces a separate
// Inventory.reserve invocation. This is how "for each item in cart,
// reserve inventory" is expressed declaratively.
//
// Example with 3 cart items:
//   1. Cart.checkout completes with Success
//   2. Where clause returns 3 bindings (widget, gadget, doodad)
//   3. Engine generates 3 Inventory.reserve invocations
//   4. 3 sync_firings records created (one per binding, unique by binding_hash)
sync: "cart-inventory-reserve": {
	// Flow scoping ensures we only process items from the same user flow
	scope: "flow"

	// When: Trigger on successful cart checkout
	// The case field matches the typed output variant (Story 1.3)
	when: {
		action: "Cart.checkout"
		event:  "completed"
		case:   "Success"
		bind: {
			order_id:    "output.order_id"
			total_items: "output.total_items"
		}
	}

	// Where: Query all items in the cart
	// Returns a SET of bindings - one per CartItem row matching the filter.
	// Multi-binding pattern: N rows → N invocations
	where: {
		from:   "CartItem"
		filter: "flow_token = :flow_token"
		bind: {
			item_id:  "item_id"
			quantity: "quantity"
		}
	}

	// Then: Generate one Inventory.reserve invocation per binding
	// With 3 cart items, this fires 3 times with different bound values
	then: {
		action: "Inventory.reserve"
		args: {
			item_id:  "bound.item_id"
			quantity: "bound.quantity"
		}
	}
}

// Sync Rule: handle-insufficient-stock
//
// PATTERN: Error matching (MEDIUM-2)
// When inventory reservation fails due to insufficient stock,
// propagate the error back to the cart context.
//
// This demonstrates matching on typed error variants:
//   - The when.case matches "InsufficientStock" (not just any error)
//   - Error fields (item_id, available, requested) are extracted into bindings
//   - Downstream action receives structured error information
//
// Error handling philosophy:
//   - Errors are typed action outputs, not exceptions
//   - Sync rules match error cases and propagate them
//   - No rollback needed - state is consistent via coordination
sync: "handle-insufficient-stock": {
	scope: "flow"

	// When: Trigger on InsufficientStock error from Inventory.reserve
	// Matches the typed error variant defined in inventory.concept.cue
	when: {
		action: "Inventory.reserve"
		event:  "completed"
		case:   "InsufficientStock" // Specific error variant, not generic failure
		bind: {
			item_id:   "output.item_id"
			available: "output.available"
			requested: "output.requested"
		}
	}

	// Where: Omitted - no state query needed
	// Error bindings from when clause provide all necessary context

	// Then: Notify cart of the stock failure
	// In a real system, this might trigger UI notification, logging, etc.
	then: {
		action: "Cart.notifyStockFailure"
		args: {
			item_id:   "bound.item_id"
			available: "bound.available"
			requested: "bound.requested"
			reason:    "insufficient_stock"
		}
	}
}

// Sync Rule: web-cart-integration
//
// PATTERN: Cross-concept coordination
// When a web request targets the checkout endpoint, initiate cart checkout.
//
// This demonstrates external trigger integration:
//   - Web.request captures inbound HTTP-like requests
//   - Sync rule translates to domain action (Cart.checkout)
//   - Flow token propagates through the entire request lifecycle
sync: "web-cart-checkout": {
	scope: "flow"

	// When: External checkout request received
	when: {
		action: "Web.request"
		event:  "completed"
		case:   "Success"
		bind: {
			request_id: "output.request_id"
		}
	}

	// Where: Match requests targeting checkout path
	// Uses Request state to filter by path
	where: {
		from:   "Request"
		filter: "request_id = :request_id AND path = '/checkout' AND method = 'POST'"
		bind: {
			cart_id: "flow_token" // Cart identified by flow
		}
	}

	// Then: Initiate cart checkout
	then: {
		action: "Cart.checkout"
		args: {
			cart_id: "bound.cart_id"
		}
	}
}
