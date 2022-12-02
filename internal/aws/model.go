package aws

import (
	identityStoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	orgTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
)

type CapabilitySso struct {
	RootId         string
	AwsAccountId   string
	AwsAccountName string
}

type GetAccountsMissingCapabilityPermissionSetResponse struct {
	Account *orgTypes.Account
	Group   *identityStoreTypes.Group
}
