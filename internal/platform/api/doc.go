// Package api is the HTTP transport layer shared by every feature: the
// typed handler wrapper, request binding, strict JSON decoding, validation,
// response encoding, error mapping and the middleware stack.
//
// Feature handlers are functions of the form
//
//	func(ctx context.Context, req Request) (Response, error)
//
// registered with Handle. Everything else in this package exists to make
// those functions the only HTTP-specific code a feature needs.
package api
