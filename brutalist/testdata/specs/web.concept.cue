// Web concept for HTTP-like request/response coordination.
// Manages external interactions with request/response semantics,
// tracking pending requests and their responses.
package specs

concept: Web: {
	purpose: "HTTP-like request/response coordination for external interactions"

	state: Request: {
		request_id: string
		path:       string
		method:     string
		body:       string
		flow_token: string
		completed:  bool
	}

	state: Response: {
		request_id: string
		status:     int
		body:       string
	}

	action: request: {
		args: {
			path:   string
			method: string
			body:   string
		}
		outputs: [{
			case:   "Success"
			fields: {request_id: string}
		}]
	}

	action: respond: {
		args: {
			request_id: string
			status:     int
			body:       string
		}
		outputs: [
			{
				case:   "Success"
				fields: {}
			},
			{
				case:   "RequestNotFound"
				fields: {request_id: string}
			},
			{
				case:   "AlreadyResponded"
				fields: {request_id: string}
			},
		]
	}

	operational_principle: """
		Request creates a pending request record with unique request_id.
		Request stores path, method, body, and flow_token for tracing.
		Respond completes request with status code and response body.
		Respond fails with RequestNotFound if request_id is invalid.
		Respond fails with AlreadyResponded if request already completed.
		Each request can only be responded to once.
		"""
}
