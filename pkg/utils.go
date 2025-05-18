package pkg

// type for setting id key in request context
type idstring string

// value used as a key for setting id in request context
const RequestID idstring = "request_id"
