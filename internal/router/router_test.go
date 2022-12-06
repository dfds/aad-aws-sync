package router

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.dfds.cloud/aad-aws-sync/internal/handlerstest"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkatest"
	"go.uber.org/zap"
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
	tc.ctx = context.WithValue(tc.ctx, ContextKeyLogger, zap.NewNop())
	tc.ctx = context.WithValue(tc.ctx, ContextKeyKafkaConsumer, tc.mockConsumer)
	tc.ctx = context.WithValue(tc.ctx, ContextKeyEventHandlers, tc.mockEventHandlers)

	return tc
}

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
	testCapabilityCreatedMessageValueWithUnsupportedVersion = `
{
   "version": "unsupported",
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

func assertHandledEvent(t *testing.T, mockEventHandlers *handlerstest.MockEventHandlers,
	expectedMessage kafka.Message, expectedEventName, expectedEventVersion string) {
	handledEvent := mockEventHandlers.Calls[0].Arguments.Get(1).(kafkamsgs.Event)
	assert.EqualValues(t, expectedMessage, handledEvent.Message)
	assert.Equal(t, expectedEventName, handledEvent.Name)
	assert.Equal(t, expectedEventVersion, handledEvent.Version)
}

func assertHandledErrorEvent(t *testing.T, mockEventHandlers *handlerstest.MockEventHandlers,
	expectedMessage kafka.Message, expectedError string) {
	handledEvent := mockEventHandlers.Calls[0].Arguments.Get(1).(kafkamsgs.Event)
	handledError := mockEventHandlers.Calls[0].Arguments.Get(2).(error)
	assert.EqualValues(t, expectedMessage, handledEvent.Message)
	assert.Equal(t, expectedError, handledError.Error())
}

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
	handlerCall := tc.mockEventHandlers.On("CapabilityCreatedHandler", mock.Anything, mock.Anything).
		NotBefore(fetchMessageCall).
		Once()
	tc.mockConsumer.
		On("CommitMessages", mock.Anything, mock.Anything).
		NotBefore(handlerCall).
		Once().
		Return(nil)

	// Execute the router
	ConsumeMessages(tc.ctx)

	// Assertion expected calls
	tc.mockConsumer.AssertExpectations(t)
	tc.mockConsumer.AssertNumberOfCalls(t, "FetchMessage", 2)
	tc.mockConsumer.AssertNumberOfCalls(t, "CommitMessages", 1)
	tc.mockEventHandlers.AssertExpectations(t)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "CapabilityCreatedHandler", 1)

	// Assertions regarding the handled event
	assertHandledEvent(t, tc.mockEventHandlers, msg, "capability_created", "1")
}

func TestConsumeMessagesUnsupportedVersion(t *testing.T) {
	// Initiate test context
	tc := newTestRouterContext()

	msg := kafka.Message{
		Key:   []byte(testCapabilityCreatedMessageKey),
		Value: []byte(testCapabilityCreatedMessageValueWithUnsupportedVersion),
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
	handlerCall := tc.mockEventHandlers.On("PermanentErrorHandler", mock.Anything, mock.Anything, mock.Anything).
		NotBefore(fetchMessageCall).
		Once()
	tc.mockConsumer.
		On("CommitMessages", mock.Anything, mock.Anything).
		NotBefore(handlerCall).
		Once().
		Return(nil)

	// Execute the router
	ConsumeMessages(tc.ctx)

	// Assertion expected calls
	tc.mockConsumer.AssertExpectations(t)
	tc.mockConsumer.AssertNumberOfCalls(t, "FetchMessage", 2)
	tc.mockConsumer.AssertNumberOfCalls(t, "CommitMessages", 1)
	tc.mockEventHandlers.AssertExpectations(t)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "PermanentErrorHandler", 1)

	// Assertions regarding the handled event
	assertHandledErrorEvent(t, tc.mockEventHandlers, msg, ErrUnsupportedEventVersion.Error())
}

func TestConsumeMessagesUnsupportedEvent(t *testing.T) {
	// Initiate test context
	tc := newTestRouterContext()

	msg := kafka.Message{
		Headers: kafkamsgs.EventHeaders("unsupported_event", "1"),
		Key:     []byte("test-key-f82e4bf8-f0a4-4007-9175-5f9bb6bd96f4"),
		Value:   []byte("the event payload"),
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
		NotBefore(fetchMessageCall).
		Once().
		Return(nil)

	// Execute the router
	ConsumeMessages(tc.ctx)

	// Assertion expected calls
	tc.mockConsumer.AssertExpectations(t)
	tc.mockConsumer.AssertNumberOfCalls(t, "FetchMessage", 2)
	tc.mockConsumer.AssertNumberOfCalls(t, "CommitMessages", 1)
	tc.mockEventHandlers.AssertExpectations(t)
	// Should not publish an error in this case
	tc.mockEventHandlers.AssertNumberOfCalls(t, "PermanentErrorHandler", 0)
}

func TestConsumeMessagesUndeterminedEventName(t *testing.T) {
	// Initiate test context
	tc := newTestRouterContext()

	msg := kafka.Message{
		Key: []byte("test-key-46b19124-f8cc-4c60-afc2-51ab8ad05c92"),
		// No way to determine event name and version from payload
		// or headers.
		Value: []byte("there is no event metadata here"),
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
	handlerCall := tc.mockEventHandlers.On("PermanentErrorHandler", mock.Anything, mock.Anything, mock.Anything).
		NotBefore(fetchMessageCall).
		Once()
	tc.mockConsumer.
		On("CommitMessages", mock.Anything, mock.Anything).
		NotBefore(handlerCall).
		Once().
		Return(nil)

	// Execute the router
	ConsumeMessages(tc.ctx)

	// Assertion expected calls
	tc.mockConsumer.AssertExpectations(t)
	tc.mockConsumer.AssertNumberOfCalls(t, "FetchMessage", 2)
	tc.mockConsumer.AssertNumberOfCalls(t, "CommitMessages", 1)
	tc.mockEventHandlers.AssertExpectations(t)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "PermanentErrorHandler", 1)

	// Assertions regarding the handled event
	assertHandledErrorEvent(t, tc.mockEventHandlers, msg, "unable to determine an event name")
}

func TestConsumeMessagesCanceled(t *testing.T) {
	// Initiate test context
	tc := newTestRouterContext()

	// Mock expected calls
	tc.mockConsumer.
		On("FetchMessage", mock.Anything, mock.Anything).
		Once().
		Return(kafka.Message{}, context.Canceled)

	// Execute the router
	ConsumeMessages(tc.ctx)

	// Assertion expected calls
	tc.mockConsumer.AssertExpectations(t)
	tc.mockConsumer.AssertNumberOfCalls(t, "FetchMessage", 1)
	tc.mockConsumer.AssertNumberOfCalls(t, "CommitMessages", 0)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "CapabilityCreatedHandler", 0)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "PermanentErrorHandler", 0)
}

func TestConsumeMessagesFetchError(t *testing.T) {
	// Initiate test context
	tc := newTestRouterContext()

	// Mock expected calls
	tc.mockConsumer.
		On("FetchMessage", mock.Anything, mock.Anything).
		Once().
		Return(kafka.Message{}, errors.New("something happened"))

	// Execute the router
	ConsumeMessages(tc.ctx)

	// Assertion expected calls
	tc.mockConsumer.AssertExpectations(t)
	tc.mockConsumer.AssertNumberOfCalls(t, "FetchMessage", 1)
	tc.mockConsumer.AssertNumberOfCalls(t, "CommitMessages", 0)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "CapabilityCreatedHandler", 0)
	tc.mockEventHandlers.AssertNumberOfCalls(t, "PermanentErrorHandler", 0)
}
