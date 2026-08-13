// Package safe provides a small helper for making concurrent code
// resilient to panics that would otherwise crash the whole process.
package safe

import (
	"log"
	"runtime/debug"
)

// Recover must be called via defer at the top of any goroutine body that
// isn't the single per-request goroutine net/http itself already protects
// with its own recover. Without it, a panic in a spawned goroutine —
// e.g. inside golang.org/x/sync/errgroup.Group.Go, which deliberately does
// not recover panics, by its own documentation — propagates uncaught and
// crashes the entire process. In a container, that process is PID 1: one
// bad request panicking in a detector or an image codec kills the whole
// server instead of just failing that one request.
func Recover(context string) {
	if r := recover(); r != nil {
		log.Printf("recovered from panic in %s: %v\n%s", context, r, debug.Stack())
	}
}
