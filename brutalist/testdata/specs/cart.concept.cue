// Cart concept for e-commerce shopping cart functionality.
// Manages shopping cart items for a user session, supporting
// add, remove, and checkout operations with typed error variants.
package specs

concept: Cart: {
	purpose: "Manage shopping cart items and checkout for user sessions"

	state: CartItem: {
		cart_id:    string
		item_id:    string
		quantity:   int
		flow_token: string // Links cart items to their originating flow
	}

	action: addItem: {
		args: {
			cart_id:  string
			item_id:  string
			quantity: int
		}
		outputs: [
			{
				case:   "Success"
				fields: {item_id: string, new_quantity: int}
			},
			{
				case:   "InvalidQuantity"
				fields: {message: string}
			},
		]
	}

	action: removeItem: {
		args: {
			cart_id: string
			item_id: string
		}
		outputs: [
			{
				case:   "Success"
				fields: {}
			},
			{
				case:   "ItemNotFound"
				fields: {item_id: string}
			},
		]
	}

	action: checkout: {
		args: {
			cart_id: string
		}
		outputs: [
			{
				case:   "Success"
				fields: {order_id: string, total_items: int}
			},
			{
				case:   "EmptyCart"
				fields: {}
			},
		]
	}

	// Notification action for stock-related failures during checkout
	// Called by handle-insufficient-stock sync rule
	action: notifyStockFailure: {
		args: {
			item_id:   string
			available: int
			requested: int
			reason:    string
		}
		outputs: [{
			case:   "Success"
			fields: {}
		}]
	}

	operational_principle: """
		Adding an item that exists increases quantity.
		Adding an item that does not exist creates a new cart entry.
		Adding with quantity <= 0 returns InvalidQuantity error.
		Removing an item that exists removes the cart entry.
		Removing an item that does not exist returns ItemNotFound error.
		Checkout with empty cart returns EmptyCart error.
		Checkout with items returns order_id and total_items count.
		NotifyStockFailure records when items cannot be reserved due to stock issues.
		"""
}
