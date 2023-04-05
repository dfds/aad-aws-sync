package event

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.dfds.cloud/aad-aws-sync/internal/event/model"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	assert.NotNil(t, r)
}

func TestRegistry_GetHandler(t *testing.T) {
	r := NewRegistry()
	r.Register("new_capability", func(ctx context.Context, event model.HandlerContext) error {
		return nil
	})

	f := r.GetHandler("new_capability")
	assert.NotNil(t, f)
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	r.Register("new_capability", func(ctx context.Context, event model.HandlerContext) error {
		return nil
	})
}
