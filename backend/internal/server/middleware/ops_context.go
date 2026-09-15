package middleware

// Ops error-log context keys shared with the ops error logger middleware.
//
// The logger builds its entry from these gin keys after the response is written.
// A middleware that rejects a request before any handler runs must still set the
// model it rejected, otherwise the error log records an empty model and nobody
// can tell which model the caller actually asked for.
const (
	OpsModelKey  = "ops_model"
	OpsStreamKey = "ops_stream"
)
