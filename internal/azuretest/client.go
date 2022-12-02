package azuretest

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
)

// TODO autogenerate these with mockery

// MockAzureClient is used to mock the AzureClient interface.
type MockAzureClient struct {
	mock.Mock
}

func (c *MockAzureClient) CreateAdministrativeUnitGroup(ctx context.Context, requestPayload azure.CreateAdministrativeUnitGroupRequest) (*azure.CreateAdministrativeUnitGroupResponse, error) {
	args := c.Called(ctx, requestPayload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*azure.CreateAdministrativeUnitGroupResponse), args.Error(1)
}
