// Package chain provides utilities for composing HTTP middleware chains.
package chain

import (
	"net/http"
)

// Middleware is the standard middleware function signature.
type Middleware func(http.Handler) http.Handler

// Chain combines multiple middleware into a single middleware.
// Middlewares are applied in order, with the first middleware wrapping outermost.
// Example: Chain(A, B, C) produces A(B(C(handler))).
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		// Apply middlewares in reverse order so the first middleware wraps outermost.
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Group represents a reusable set of middleware.
type Group struct {
	middlewares []Middleware
}

// NewGroup creates a new middleware group with the given middlewares.
func NewGroup(middlewares ...Middleware) *Group {
	return &Group{
		middlewares: middlewares,
	}
}

// Use appends one or more middlewares to the group.
func (g *Group) Use(middlewares ...Middleware) *Group {
	g.middlewares = append(g.middlewares, middlewares...)
	return g
}

// Then returns the middleware chain that wraps the final handler.
func (g *Group) Then(handler http.Handler) http.Handler {
	return Chain(g.middlewares...)(handler)
}

// ThenFunc returns the middleware chain that wraps the final handler function.
func (g *Group) ThenFunc(fn http.HandlerFunc) http.Handler {
	return g.Then(fn)
}

// Middleware returns the group as a single middleware function.
func (g *Group) Middleware() Middleware {
	return Chain(g.middlewares...)
}

// Clone creates a copy of the group that can be modified independently.
func (g *Group) Clone() *Group {
	cloned := make([]Middleware, len(g.middlewares))
	copy(cloned, g.middlewares)
	return &Group{
		middlewares: cloned,
	}
}

// Extend returns a new group with additional middlewares appended.
// The original group is not modified.
func (g *Group) Extend(middlewares ...Middleware) *Group {
	return g.Clone().Use(middlewares...)
}
