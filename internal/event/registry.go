package event

import (
	"context"
	"go.dfds.cloud/aad-aws-sync/internal/event/model"
)

type HandlerFunc func(ctx context.Context, event model.HandlerContext) error

type Registry struct {
	handlers map[string]HandlerFunc
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: map[string]HandlerFunc{},
	}
}

func (r *Registry) Register(eventName string, f HandlerFunc) {
	r.handlers[eventName] = f
}

func (r *Registry) GetHandler(eventName string) HandlerFunc {
	return r.handlers[eventName]
}
