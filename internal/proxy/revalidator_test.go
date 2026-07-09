/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"context"
	"testing"
)

func TestNoOpRevalidator(t *testing.T) {
	r := &NoOpRevalidator{}
	if err := r.Revalidate(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
