package httpwares

import "net/http"

// Middleware is signature of all http server-side middleware.
type Middleware func(http.Handler) http.Handler
