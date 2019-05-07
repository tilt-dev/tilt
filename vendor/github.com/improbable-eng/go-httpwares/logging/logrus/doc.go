/*
`http_logrus` is a HTTP logging middleware for the Logrus logging stack.

It provides both middleware (server-side) and tripperware (client-side) for logging HTTP requests using a user-provided
`logrus.Entry`.


Middleware server-side logging

The middleware also embeds a request-field scoped `logrus.Entry` (with fields from `ctxlogrus`) inside the `context.Context`
of the `http.Request` that is passed to the executing `http.Handler`. That `logrus.Entry` can be easily extracted using
It accepts a user-configured `logrus.Entry` that will be used for logging completed HTTP calls. The same
`logrus.Entry` will be used for logging completed gRPC calls, and be populated into the `context.Context` passed into
HTTP handler code. To do that, use the `Extract` method (see example below).

The middlewarerequest will be logged at a level indicated by `WithLevels` options, and an example JSON-formatted
log message will look like:

	{
	"@timestamp:" "2006-01-02T15:04:05Z07:00",
	"@level": "info",
	"my_custom.my_string": 1337,
	"custom_tags.string": "something",
	"http.handler.group": "my_service",
	"http.host": "something.local",
	"http.proto_major": 1,
	"http.request.length_bytes": 0,
	"http.status": 201,
	"http.time_ms": 0.095,
	"http.url.path": "/someurl",
	"msg": "handled",
	"peer.address": "127.0.0.1",
	"peer.port": "59141",
	"span.kind": "server",
	"system": "http"
	}

Tripperware client-side logging

The tripperware uses any `ctxlogrus` to create a request-field scoped `logrus.Entry`. The key one is the `http.call.service`
which by default is auto-detected from the domain but can be overwritten by the `ctxlogrus` initialization.

Most requests and responses won't be loged. By default only client-side connectivity  and 5** responses cause
the outbound requests to be logged, but that can be customized using `WithLevels` and `WithConnectivityError` options. A
typical log message for client side will look like:

	{
	"@timestamp:" "2006-01-02T15:04:05Z07:00",
	"@level": "debug",
	"http.call.service": "googleapis",
	"http.host": "calendar.googleapis.com",
	"http.proto_major": 1,
	"http.request.length_bytes": 0,
	"http.response.length_bytes": 176,
	"http.status": 201,
	"http.time_ms": 4.654,
	"http.url.path": "/someurl",
	"msg": "request completed",
	"span.kind": "client",
	"system": "http"
	}

You can use `Extract` to log into a request-scoped `logrus.Entry` instance in your handler code.
Additional tags to the logger can be added using `ctxlogrus`.

HTTP Library logging

The `http.Server` takes a logger command. You can use the `AsHttpLogger` to take a user-scoped `logrus.Entry` and log
connectivity or low-level HTTP errors (e.g. TLS handshake problems, badly formed requests etc).

Please see examples and tests for examples of use.
*/
package http_logrus
