package azuretest

// MockAzureClient is used to mock the AzureClient interface.
type MockAzureClient struct {
	CreateGroupCalls []string
	CreateGroupMock  func(string) (string, error)
}

func (c *MockAzureClient) CreateGroup(name string) (string, error) {
	c.CreateGroupCalls = append(c.CreateGroupCalls, name)
	return c.CreateGroupMock(name)
}
