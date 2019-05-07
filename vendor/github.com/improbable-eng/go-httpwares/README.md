# :stew: Go HTTP Wares 

[![Travis Build](https://travis-ci.org/improbable-eng/go-httpwares.svg?branch=master)](https://travis-ci.org/improbable-eng/go-httpwares)
[![Go Report Card](https://goreportcard.com/badge/github.com/improbable-eng/go-httpwares)](https://goreportcard.com/report/github.com/improbable-eng/go-httpwares)
[![GoDoc](http://img.shields.io/badge/GoDoc-Reference-blue.svg)](https://godoc.org/github.com/improbable-eng/go-httpwares)
[![SourceGraph](https://sourcegraph.com/github.com/improbable-eng/go-httpwares/-/badge.svg)](https://sourcegraph.com/github.com/improbable-eng/go-httpwares/?badge)
[![codecov](https://codecov.io/gh/improbable-eng/go-httpwares/branch/master/graph/badge.svg)](https://codecov.io/gh/improbable-eng/go-httpwares)
[![Apache 2.0 License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![quality: alpha](https://img.shields.io/badge/quality-alpha-orange.svg)](#status)

Client and Server HTTP middleware libraries leveraging Go's `Context`-based `net/http` library.

## Why?

Just to clarify, these libraries are *not* yet another Go HTTP router, nor is it an *REST client library*.
They are meant to interop with anything that `net/http` supports, without requiring much re-jigging of code.

The reason to include both server-side (middlewares) and client-side (tripperwares) is because Go HTTP servers often make
HTTP requests themselves when handling an inbound request. As such, in order to make services debuggable
in production, it is crucial to have the same sort of monitoring, logging and tracing available for both inbound and outbound
requests. Moreover, some values (tracing, auth tokens) need to be passed from input to the output.

These libraries are meant as excellent companions for interceptors of [`github.com/grpc-ecosystem/go-grpc-middleware`](https://github.com/grpc-ecosystem/go-grpc-middleware) making it easy to build combined gRPC/HTTP Golang servers.

## Wares

### Middlewares (server-side)

Middlewares adhere to `func (http.Handler) http.Handler` signature. I.e. it is a handler that accept a handler.

This means that the composition purely `net/http`-based. This means that you can use it with [`echo`](https://github.com/labstack/echo), [`martini`](https://github.com/go-martini/martini),
or a home-grown router/framework of choice, worst-case you'll slide it between the `http.Server` and the `http.Handler` function of the framework.

The same composition is adopted in [chi](https://github.com/pressly/chi) and [goji](https://github.com/goji/goji), which means that
you can use any middleware that's compatible with them, e.g.:
  * [gorilla/csrf](https://github.com/gorilla/csrf) -  Cross Site Request Forgery (CSRF) prevention middleware
  * [gowares/cors](https://github.com/goware/cors) - Cross-Origin Resource Sharing handling middleware
  * [chi/requestid](https://github.com/pressly/chi/blob/master/middleware/request_id.go) - request-id generator
etc.

As such, this repository focuses on debugability of handlers. The crucial package here is [`http_ctxtags`](tags/README.md)
which propagates a set of key-value pairs through different middlewares in the `http.Request.Context` for both writing and reading.
This means you get a canonical set of metadata for logging, monitoring and tracing of your inbound requests. Additionally, it
allows assigning a name to a group of handlers (e.g. `auth`), and names to individual handlers (e.g. `token_exchange`).

The middlewares provided in this repo are:
 * Monitoring
   * [monitoring/prometheus](monitoring/prometheus) - [Prometheus](https://prometheus.io/) server-side monitoring broken down by handler group and name.
 * Tracing
   * [tracing/debug](tracing/debug)  - `/debug/request` page for server-side HTTP request handling, allowing you to inspect failed requests, inbound headers etc.
   * [tracing/opentracing](tracing/opentracing) - server-side request [Opentracing](http://opentracing.io/) middleware that is tags-aware and supports client-side propagation
 * Logging
   * [logging/logrus](logging/logrus) - a [Logrus](https://github.com/sirupsen/logrus)-based logger for HTTP requests:
      * injects a request-scoped `logrus.Entry` into the `http.Request.Context` for further logging
      * optionally supports logging of inbound request content and response contents in raw or JSON format


### Tripperware (client-side)

Tripperwares adhere to `func (http.RoundTripper) http.RoundTripper` signature, hence the name. As such they are used as
a `Transport` for `http.Client`, and can wrap other transports.

This means that the composition is purely `net/http`-based. Since there are few (if any) libraries for this, the repository
will have multiple useful libraries for making external calls.

The crucial package here is again `http_ctxtags`(tags/README.md) as it introduces a concept of a *service name*. This is
either user-specified or automatically detected from the URL (for external calls), and is used as a key indicator in all
other debugability handlers.

The tripperwares provided in this repo are:
 * Monitoring
   * [metrics/prometheus](metrics/prometheus) - [Prometheus](https://prometheus.io/) client-side monitoring broken down by service name
 * Tracing
   * [tracing/debug](tracing/debug) - `/debug/request` page for client-side HTTP request debugging, allowing  you to inspect failed requests, outbound headers, payload sizes etc etc.
   * [tracing/opentracing](tracing/opentracing) - client-side request [Opentracing](http://opentracing.io/) middleware that is tags-aware and supports propagation of traces from server-side middleware
 * Logging
   * [logging/logrus](logging/logrus) - a [Logrus](https://github.com/sirupsen/logrus)-based logger for HTTP calls requests:
      * optionally supports logging of inbound request content and response contents in raw or JSON format
 * Retry
   * [retry](retry) - a simple retry-middleware that retries on connectivity and bad response errors.

### Generic building blocks

All the libraries included here are meant to be minimum-dependency based. As such the root of the package, `http_wares`
contains helper libraries that allow for chaining and wrapping `http.ResponseWriter` objects. See  [documentation](DOC.md) for more.

## Status

This code is **experimental** and still considered a work in progress.
It is meant to be the underpinnings of the Go HTTP stack at [Improbable](https://improbable.io).

Additional tooling will be added, and contributions are welcome.

## License

`go-httpwares` is released under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
