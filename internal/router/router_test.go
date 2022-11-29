package router

import (
	"context"
	"io"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
	"go.dfds.cloud/aad-aws-sync/internal/handlerstest"
	"go.dfds.cloud/aad-aws-sync/internal/kafkatest"
)

type testRouterContext struct {
	ctx context.Context

	mockConsumer      *kafkatest.MockKafkaConsumer
	mockEventHandlers *handlerstest.MockEventHandlers
}

func newTestRouterContext() *testRouterContext {
	ctx, _ := context.WithCancel(context.Background())
	tc := &testRouterContext{
		ctx:               ctx,
		mockConsumer:      new(kafkatest.MockKafkaConsumer),
		mockEventHandlers: new(handlerstest.MockEventHandlers),
	}

	// Initiate the context
	tc.ctx = context.WithValue(tc.ctx, ContextKeyKafkaConsumer, tc.mockConsumer)
	tc.ctx = context.WithValue(tc.ctx, ContextKeyEventHandlers, tc.mockEventHandlers)

	return tc
}

// TODO test canceling of context

// TODO test error fetching message, is that retried if so how much?

// TODO test undetermined event name

// TODO test unhandled event, no error

// TODO test proper version of an event
// TODO test unhandled versions of an event

const (
	testCapabilityCreatedMessageKey   string = "c1790f03-8b8c-4c85-98b4-86c00b588a1e"
	testCapabilityCreatedMessageValue        = `
{
   "version": "1",
   "eventName": "capability_created",
   "x-correlationId": "40e9f58d-5520-45d4-b4e8-b284e2121715",
   "x-sender": "CapabilityService.WebApi, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null",
   "payload": {
     "capabilityId": "c1790f03-8b8c-4c85-98b4-86c00b588a1e",
     "capabilityName": "Sandbox-test"
   }
}
`
)

func TestConsumeMessages(t *testing.T) {
	// Initiate test context
	tc := newTestRouterContext()

	msg := kafka.Message{
		Key:   []byte(testCapabilityCreatedMessageKey),
		Value: []byte(testCapabilityCreatedMessageValue),
	}

	// Mock expected calls
	fetchMessageCall := tc.mockConsumer.
		On("FetchMessage", mock.Anything, mock.Anything).
		Once().
		Return(msg, nil)
	tc.mockConsumer.
		On("FetchMessage", mock.Anything, mock.Anything).
		Once().
		Return(kafka.Message{}, io.EOF)
	tc.mockConsumer.
		On("CommitMessages", mock.Anything, mock.Anything).
		Once().
		Return(nil)

	tc.mockEventHandlers.On("CapabilityCreatedHandler", mock.Anything, mock.Anything).
		NotBefore(fetchMessageCall).
		Once()

	// Execute the handler
	ConsumeMessages(tc.ctx)

	// Assertion expected calls
	tc.mockConsumer.AssertExpectations(t)
	tc.mockConsumer.AssertNumberOfCalls(t, "FetchMessage", 2)
	tc.mockConsumer.AssertNumberOfCalls(t, "CommitMessages", 1)
	tc.mockEventHandlers.AssertExpectations(t)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "CapabilityCreatedHandler", 1)

	// TODO assertions about the handler call
}
