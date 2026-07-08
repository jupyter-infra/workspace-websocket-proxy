/*
Copyright (c) 2026 Jupyter Infrastructure
Distributed under the terms of the MIT license
*/

package proxy

import "context"

// Revalidator defines the interface for periodic session re-validation.
// This is scaffolding for future implementation. When implemented, the proxy
// will periodically call the auth middleware to verify the user still has
// access to the workspace.
type Revalidator interface {
	// Revalidate checks whether the session is still authorized.
	// Returns nil if the session is valid, or an error if it should be terminated.
	Revalidate(ctx context.Context) error
}

// NoOpRevalidator is the default implementation that always returns valid.
type NoOpRevalidator struct{}

// Revalidate always returns nil (session always valid).
func (n *NoOpRevalidator) Revalidate(_ context.Context) error {
	return nil
}
