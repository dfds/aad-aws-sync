package handlers

// TODO write a test here

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/azuretest"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkatest"
)

// TODO test the json decoding failing
// TODO test the create group request temp error
// TODO test a create group request permanent error

// TODO test write response temp error
// TODO test write response network error
// TODO test write response permanent error

const (
	testAzureParentAdministrativeUnitId  string = "test-parent-au-id-40fe95dc-2b00-4c88-a710-768d726b1ba1"
	testAzureCreatedAdministrativeUnitId        = "test-created-au-id-c3e35512-77fe-4736-acfd-aea79d9c775f"
)

const (
	testCapabilityName           string = "Sandbox-test"
	testCapabilityId                    = "c1790f03-8b8c-4c85-98b4-86c00b588a1e"
	testCapabilityCreatedMessage        = `
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

func TestCapabilityCreatedHandler(t *testing.T) {
	// Initiate test context
	mockAzureClient := new(azuretest.MockAzureClient)
	mockProducer := new(kafkatest.MockKafkaProducer)
	mockErrorProducer := new(kafkatest.MockKafkaProducer)

	ctx, _ := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, ContextKeyAzureClient, mockAzureClient)
	ctx = context.WithValue(ctx, ContextKeyAzureParentAdministrativeUnitID, testAzureParentAdministrativeUnitId)
	ctx = context.WithValue(ctx, ContextKeyKafkaProducer, mockProducer)
	ctx = context.WithValue(ctx, ContextKeyKafkaErrorProducer, mockErrorProducer)

	msg := kafkamsgs.Event{
		Name:    kafkamsgs.EventNameCapabilityCreated,
		Version: kafkamsgs.Version1,
		Message: kafka.Message{
			Value: []byte(testCapabilityCreatedMessage),
		},
	}

	// Mock expected calls
	createGroupCall := mockAzureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).Return(&azure.CreateAdministrativeUnitGroupResponse{
		ID: testAzureCreatedAdministrativeUnitId,
	}, nil)

	mockProducer.On("WriteMessages", mock.Anything, mock.Anything).NotBefore(createGroupCall).Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(ctx, msg)

	// Assertion expected calls
	mockAzureClient.AssertExpectations(t)
	mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 1)
	mockProducer.AssertExpectations(t)
	mockProducer.AssertNumberOfCalls(t, "WriteMessages", 1)
	mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assetions regarding the request to create the AD group
	createRequest := mockAzureClient.Calls[0].Arguments.Get(1).(azure.CreateAdministrativeUnitGroupRequest)

	assert.Equal(t, "CI_SSU_Cap - "+testCapabilityId, createRequest.DisplayName)
	assert.Equal(t, "ci-ssu_cap_"+testCapabilityId, createRequest.MailNickname)
	assert.Equal(t, testAzureParentAdministrativeUnitId, createRequest.ParentAdministrativeUnitId)
	assert.False(t, createRequest.MailEnabled)
	assert.True(t, createRequest.SecurityEnabled)

	// Assertions regarding the result message
	resultMessage := mockProducer.Calls[0].Arguments.Get(1).(kafka.Message)
	assert.EqualValues(t, testAzureCreatedAdministrativeUnitId, resultMessage.Key)
	assert.JSONEq(t, `{"azureAdGroupId": "`+testAzureCreatedAdministrativeUnitId+`", "capabilityName": "Sandbox-test" }`, string(resultMessage.Value))
	assert.EqualValues(t, []protocol.Header{{Key: "Version", Value: []byte("1")}, {Key: "Event Name", Value: []byte("azure_ad_group_created")}}, resultMessage.Headers)
}

func assertWrittenErrorMessage(t *testing.T, mockErrorProducer *kafkatest.MockKafkaProducer, originalMessage kafka.Message, expectedError string) {
	errorMessage := mockErrorProducer.Calls[0].Arguments.Get(1).(kafka.Message)
	assert.EqualValues(t, originalMessage.Key, errorMessage.Key)
	assert.JSONEq(t, string(originalMessage.Value), string(errorMessage.Value))
	assert.EqualValues(t, append(originalMessage.Headers,
		protocol.Header{Key: "Error", Value: []byte(expectedError)}),
		errorMessage.Headers)
}

func TestCapabilityCreatedHandlerFailureToDecode(t *testing.T) {
	// Initiate test context
	mockAzureClient := new(azuretest.MockAzureClient)
	mockProducer := new(kafkatest.MockKafkaProducer)
	mockErrorProducer := new(kafkatest.MockKafkaProducer)

	ctx, _ := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, ContextKeyAzureClient, mockAzureClient)
	ctx = context.WithValue(ctx, ContextKeyAzureParentAdministrativeUnitID, testAzureParentAdministrativeUnitId)
	ctx = context.WithValue(ctx, ContextKeyKafkaProducer, mockProducer)
	ctx = context.WithValue(ctx, ContextKeyKafkaErrorProducer, mockErrorProducer)

	msg := kafkamsgs.Event{
		Name:    kafkamsgs.EventNameCapabilityCreated,
		Version: kafkamsgs.Version1,
		Message: kafka.Message{
			Value: []byte("\"invalid message json\""),
		},
	}

	// Mock expected calls
	mockErrorProducer.On("WriteMessages", mock.Anything, mock.Anything).Return(nil)

	// Execute the handler
	CapabilityCreatedHandler(ctx, msg)

	// Assertion expected calls
	mockErrorProducer.AssertExpectations(t)
	mockErrorProducer.AssertNumberOfCalls(t, "WriteMessages", 1)
	mockAzureClient.AssertNumberOfCalls(t, "CreateAdministrativeUnitGroup", 0)
	mockProducer.AssertNumberOfCalls(t, "WriteMessages", 0)

	// Assert the expected error
	assertWrittenErrorMessage(t, mockErrorProducer, msg.Message,
		"json: cannot unmarshal string into Go value of type kafkamsgs.CapabilityCreatedMessage")
}
