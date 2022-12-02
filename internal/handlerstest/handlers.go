package handlerstest

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
)

type MockEventHandlers struct {
	mock.Mock
}

func (h *MockEventHandlers) PermanentErrorHandler(ctx context.Context, event kafkamsgs.Event, err error) {
	h.Called(ctx, event, err)
}

func (h *MockEventHandlers) CapabilityCreatedHandler(ctx context.Context, event kafkamsgs.Event) {
	h.Called(ctx, event)
}
