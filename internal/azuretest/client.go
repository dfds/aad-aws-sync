package azuretest

import (
	"context"

	"go.dfds.cloud/aad-aws-sync/internal/azure"
)

// MockAzureClient is used to mock the AzureClient interface.
type MockAzureClient struct {
	CreateAdministrativeUnitGroupRequestCalls []azure.CreateAdministrativeUnitGroupRequest
	CreateAdministrativeUnitGroupRequestMock  func(context.Context, azure.CreateAdministrativeUnitGroupRequest) (*azure.CreateAdministrativeUnitGroupResponse, error)
}

func (c *MockAzureClient) CreateAdministrativeUnitGroup(ctx context.Context, requestPayload azure.CreateAdministrativeUnitGroupRequest) (*azure.CreateAdministrativeUnitGroupResponse, error) {
	c.CreateAdministrativeUnitGroupRequestCalls = append(c.CreateAdministrativeUnitGroupRequestCalls, requestPayload)
	return c.CreateAdministrativeUnitGroupRequestMock(ctx, requestPayload)
}
