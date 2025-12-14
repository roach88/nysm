// Inventory concept for stock management with reservation semantics.
// Manages inventory stock levels, supporting reserve and release
// operations with typed error variants for insufficient stock.
package specs

concept: Inventory: {
	purpose: "Manage inventory stock levels with reservation semantics"

	state: Stock: {
		item_id:   string
		available: int
		reserved:  int
	}

	state: Reservation: {
		reservation_id: string
		item_id:        string
		quantity:       int
	}

	action: reserve: {
		args: {
			item_id:  string
			quantity: int
		}
		outputs: [
			{
				case:   "Success"
				fields: {reservation_id: string}
			},
			{
				case:   "InsufficientStock"
				fields: {item_id: string, available: int, requested: int}
			},
			{
				case:   "InvalidQuantity"
				fields: {message: string}
			},
		]
	}

	action: release: {
		args: {
			reservation_id: string
		}
		outputs: [
			{
				case:   "Success"
				fields: {}
			},
			{
				case:   "ReservationNotFound"
				fields: {reservation_id: string}
			},
		]
	}

	action: setStock: {
		args: {
			item_id:  string
			quantity: int
		}
		outputs: [{
			case:   "Success"
			fields: {item_id: string, quantity: int}
		}]
	}

	operational_principle: """
		Reserve reduces available stock and tracks reservation.
		Reserve fails with InsufficientStock if available < requested.
		Reserve fails with InvalidQuantity if quantity <= 0.
		Release restores reserved stock to available.
		Release fails with ReservationNotFound if reservation_id invalid.
		SetStock establishes initial stock level for an item.
		"""
}
