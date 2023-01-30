package event_handlers

import (
	"context"
	"errors"
	"net/http"
	"syscall"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/azuretest"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkatest"
	"go.uber.org/zap"
)

const (
	testAzureParentAdministrativeUnitId  string = "test-parent-au-id-40fe95dc-2b00-4c88-a710-768d726b1ba1"
	testAzureCreatedAdministrativeUnitId        = "test-created-au-id-c3e35512-77fe-4736-acfd-aea79d9c775f"
)

const (
	testCapabilityName                string = "Sandbox-test"
	testCapabilityId                         = "c1790f03-8b8c-4c85-98b4-86c00b588a1e"
	testCapabilityCreatedMessageKey          = "c1790f03-8b8c-4c85-98b4-86c00b588a1e"
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

var testCapabilityCreatedMessage = kafkamsgs.Event{
	Name:    kafkamsgs.EventNameCapabilityCreated,
	Version: kafkamsgs.Version1,
	Message: kafka.Message{
		Value: []byte(testCapabilityCreatedMessageValue),
	},
}

type testContext struct {
	ctx context.Context

	mockAzureClient   *azuretest.MockAzureClient
	mockProducer      *kafkatest.MockKafkaProducer
	mockErrorProducer *kafkatest.MockKafkaProducer
}

func newTestContext() *testContext {
	ctx, _ := context.WithCancel(context.Background())
	tc := &testContext{
		ctx:               ctx,
		mockAzureClient:   new(azuretest.MockAzureClient),
		mockProducer:      new(kafkatest.MockKafkaProducer),
		mockErrorProducer: new(kafkatest.MockKafkaProducer),
	}

	// Initiate the context
	tc.ctx = context.WithValue(tc.ctx, ContextKeyLogger, zap.NewNop())
	tc.ctx = context.WithValue(tc.ctx, ContextKeyAzureClient, tc.mockAzureClient)
	tc.ctx = context.WithValue(tc.ctx, ContextKeyAzureParentAdministrativeUnitID, testAzureParentAdministrativeUnitId)
	tc.ctx = context.WithValue(tc.ctx, ContextKeyKafkaProducer, tc.mockProducer)
	tc.ctx = context.WithValue(tc.ctx, ContextKeyKafkaErrorProducer, tc.mockErrorProducer)

	return tc
}

func assertWrittenResultMessage(t *testing.T, mockProducer *kafkatest.MockKafkaProducer) {
	resultMessage := mockProducer.Calls[0].Arguments.Get(1).(kafka.Message)
	assert.EqualValues(t, testAzureCreatedAdministrativeUnitId, resultMessage.Key)
	assert.JSONEq(t, `{"azureAdGroupId": "`+testAzureCreatedAdministrativeUnitId+`", "capabilityName": "`+testCapabilityName+`" }`,
		string(resultMessage.Value))
	assert.EqualValues(t, []protocol.Header{
		{Key: "Version", Value: []byte("1")}, {Key: "Event Name", Value: []byte("azure_ad_group_created")}},
		resultMessage.Headers)
}

func assertWrittenErrorMessage(t *testing.T, mockErrorProducer *kafkatest.MockKafkaProducer, originalMessage kafka.Message, expectedError string) {
	errorMessage := mockErrorProducer.Calls[0].Arguments.Get(1).(kafka.Message)
	assert.EqualValues(t, originalMessage.Key, errorMessage.Key)
	assert.JSONEq(t, string(originalMessage.Value), string(errorMessage.Value))
	assert.EqualValues(t, append(originalMessage.Headers,
		protocol.Header{Key: "Error", Value: []byte(expectedError)}),
		errorMessage.Headers)
}

func TestCapabilityCreatedHandler(t *testing.T) {
	// Initiate test context
	tc := newTestContext()

	// Mock expected calls
	createGroupCall := tc.mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Return(&azure.CreateAdministrativeUnitGroupResponse{
			ID: testAzureCreatedAdministrativeUnitId,
		}, nil)

	tc.mockProducer.On("WriteMessages", mock.Anything, mock.Anything).
		NotBefore(createGroupCall).
		Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(tc.ctx, testCapabilityCreatedMessage)

	// Assertion expected calls
	tc.mockAzureClient.AssertExpectations(t)
	tc.mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 1)
	tc.mockProducer.AssertExpectations(t)
	tc.mockProducer.AssertNumberOfCalls(t, "WriteMessages", 1)
	tc.mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assetions regarding the request to create the AD group
	createRequest := tc.mockAzureClient.Calls[0].Arguments.Get(1).(azure.CreateAdministrativeUnitGroupRequest)

	assert.Equal(t, "CI_SSU_Cap - "+testCapabilityId, createRequest.DisplayName)
	assert.Equal(t, "ci-ssu_cap_"+testCapabilityId, createRequest.MailNickname)
	assert.Equal(t, testAzureParentAdministrativeUnitId, createRequest.ParentAdministrativeUnitId)
	assert.False(t, createRequest.MailEnabled)
	assert.True(t, createRequest.SecurityEnabled)

	// Assertions regarding the result message
	assertWrittenResultMessage(t, tc.mockProducer)
}

func TestCapabilityCreatedHandlerFailureToDecode(t *testing.T) {
	// Initiate test context
	tc := newTestContext()

	// Mock expected calls
	tc.mockErrorProducer.On("WriteMessages", mock.Anything, mock.Anything).Return(nil)

	// Execute the handler
	msg := kafkamsgs.Event{
		Name:    kafkamsgs.EventNameCapabilityCreated,
		Version: kafkamsgs.Version1,
		Message: kafka.Message{
			Value: []byte("\"invalid message json\""),
		},
	}
	CapabilityCreatedHandler(tc.ctx, msg)

	// Assertion expected calls
	tc.mockErrorProducer.AssertExpectations(t)
	tc.mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 1)
	tc.mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 0)
	tc.mockProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assert the expected error
	assertWrittenErrorMessage(t, tc.mockErrorProducer, msg.Message,
		"json: cannot unmarshal string into Go value of type kafkamsgs.CapabilityCreatedMessage")
}

func TestCapabilityCreatedHandlerTemporaryCreateGroupError(t *testing.T) {
	// Initiate test context
	tc := newTestContext()

	// Mock expected calls
	tc.mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Once().
		Return(nil, azure.ApiError{StatusCode: http.StatusServiceUnavailable})
	tc.mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Return(&azure.CreateAdministrativeUnitGroupResponse{
			ID: testAzureCreatedAdministrativeUnitId,
		}, nil)
	tc.mockProducer.On("WriteMessages", mock.Anything, mock.Anything).
		Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(tc.ctx, testCapabilityCreatedMessage)

	// Assertion expected calls
	tc.mockAzureClient.AssertExpectations(t)
	tc.mockErrorProducer.AssertExpectations(t)
	tc.mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 2) // retried
	tc.mockProducer.AssertNumberOfCalls(t, "WriteMessages", 1)
	tc.mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assertions regarding the result message
	assertWrittenResultMessage(t, tc.mockProducer)
}

func TestCapabilityCreatedHandlerPermanentCreateGroupError(t *testing.T) {
	// Initiate test context
	tc := newTestContext()

	// Mock expected calls
	createGroupCall := tc.mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Return(nil, azure.ApiError{StatusCode: http.StatusNotFound})
	tc.mockErrorProducer.On("WriteMessages", mock.Anything, mock.Anything).
		NotBefore(createGroupCall).
		Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(tc.ctx, testCapabilityCreatedMessage)

	// Assertion expected calls
	tc.mockAzureClient.AssertExpectations(t)
	tc.mockErrorProducer.AssertExpectations(t)
	tc.mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 1)
	tc.mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 1)
	tc.mockProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assert the expected error
	assertWrittenErrorMessage(t, tc.mockErrorProducer, testCapabilityCreatedMessage.Message,
		"Not Found")
}

func TestCapabilityCreatedHandlerTemporaryWriteMessagesError(t *testing.T) {
	// Initiate test context
	tc := newTestContext()

	// Mock expected calls
	tc.mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Return(&azure.CreateAdministrativeUnitGroupResponse{
			ID: testAzureCreatedAdministrativeUnitId,
		}, nil)
	tc.mockProducer.On("WriteMessages", mock.Anything, mock.Anything).
		Once().
		Return(context.DeadlineExceeded)
	tc.mockProducer.On("WriteMessages", mock.Anything, mock.Anything).
		Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(tc.ctx, testCapabilityCreatedMessage)

	// Assertion expected calls
	tc.mockAzureClient.AssertExpectations(t)
	tc.mockErrorProducer.AssertExpectations(t)
	tc.mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 1)
	tc.mockProducer.AssertNumberOfCalls(t, "WriteMessages", 2) // retried
	tc.mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assertions regarding the result message
	assertWrittenResultMessage(t, tc.mockProducer)
}

func TestCapabilityCreatedHandlerNetworkWriteMessagesError(t *testing.T) {
	// Initiate test context
	tc := newTestContext()

	// Mock expected calls
	tc.mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Return(&azure.CreateAdministrativeUnitGroupResponse{
			ID: testAzureCreatedAdministrativeUnitId,
		}, nil)
	tc.mockProducer.On("WriteMessages", mock.Anything, mock.Anything).
		Once().
		Return(syscall.ECONNRESET)
	tc.mockProducer.On("WriteMessages", mock.Anything, mock.Anything).
		Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(tc.ctx, testCapabilityCreatedMessage)

	// Assertion expected calls
	tc.mockAzureClient.AssertExpectations(t)
	tc.mockErrorProducer.AssertExpectations(t)
	tc.mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 1)
	tc.mockProducer.AssertNumberOfCalls(t, "WriteMessages", 2) // retried
	tc.mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assertions regarding the result message
	assertWrittenResultMessage(t, tc.mockProducer)
}

func TestCapabilityCreatedHandlerPermanentWriteMessagesError(t *testing.T) {
	// Initiate test context
	tc := newTestContext()

	// Mock expected calls
	tc.mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Return(&azure.CreateAdministrativeUnitGroupResponse{
			ID: testAzureCreatedAdministrativeUnitId,
		}, nil)
	tc.mockProducer.On("WriteMessages", mock.Anything, mock.Anything).
		Return(errors.New("a permanent error"))
	tc.mockErrorProducer.On("WriteMessages", mock.Anything, mock.Anything).
		Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(tc.ctx, testCapabilityCreatedMessage)

	// Assertion expected calls
	tc.mockAzureClient.AssertExpectations(t)
	tc.mockErrorProducer.AssertExpectations(t)
	tc.mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 1)
	tc.mockProducer.AssertNumberOfCalls(t, "WriteMessages", 1)
	tc.mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 1)

	// Assert the expected error
	assertWrittenErrorMessage(t, tc.mockErrorProducer, testCapabilityCreatedMessage.Message,
		"a permanent error")
}
