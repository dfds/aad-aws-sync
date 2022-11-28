package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identityStoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"log"
	"strings"
)

type ManageSso struct { // TODO, make sure Account & Group are only allocated once
	AwsAccounts        []*orgTypes.Account
	AwsSsoGroups       []*identityStoreTypes.Group
	awsAccountsByAlias map[string]*orgTypes.Account
	awsAccountsById    map[string]*orgTypes.Account
	awsSsoGroupById    map[string]*identityStoreTypes.Group
	awsSsoGroupByName  map[string]*identityStoreTypes.Group
}

func (m *ManageSso) GetAccountByName(val string) *orgTypes.Account {
	if val, ok := m.awsAccountsByAlias[val]; ok {
		return val
	}
	return nil
}

func (m *ManageSso) GetAccountById(val string) *orgTypes.Account {
	if val, ok := m.awsAccountsById[val]; ok {
		return val
	}
	return nil
}

func (m *ManageSso) GetGroupById(val string) *identityStoreTypes.Group {
	if val, ok := m.awsSsoGroupById[val]; ok {
		return val
	}
	return nil
}

func (m *ManageSso) GetGroupByName(val string) *identityStoreTypes.Group {
	if val, ok := m.awsSsoGroupByName[val]; ok {
		return val
	}
	return nil
}

// GetAccountsMissingCapabilityPermissionSet
// TODO: Rewrite/simplify process
func (m *ManageSso) GetAccountsMissingCapabilityPermissionSet(client *ssoadmin.Client, ssoInstanceArn string, capabilityPermissionSetArn string, ssoGroupPrefix string, awsAccountPrefix string) ([]*GetAccountsMissingCapabilityPermissionSetResponse, error) {
	var accountsWithMissingPermissionSet []*GetAccountsMissingCapabilityPermissionSetResponse
	awsAccountsProvisionedWithCapabilityPermissions, err := GetAccountsWithProvisionedPermissionSet(client, ssoInstanceArn, capabilityPermissionSetArn)
	if err != nil {
		return accountsWithMissingPermissionSet, err
	}

	awsAccountWithPermissionSet := make(map[string]int)
	for _, accountId := range awsAccountsProvisionedWithCapabilityPermissions {
		awsAccountWithPermissionSet[accountId] = 1
	}

	for _, acc := range m.AwsAccounts {
		if _, ok := awsAccountWithPermissionSet[*acc.Id]; !ok {
			group := m.GetGroupByName(fmt.Sprintf("%s %s", ssoGroupPrefix, RemoveAccountPrefix(awsAccountPrefix, *acc.Name)))
			if group != nil {
				accountsWithMissingPermissionSet = append(accountsWithMissingPermissionSet, &GetAccountsMissingCapabilityPermissionSetResponse{
					Account: acc,
					Group:   group,
				})
			}

		}
	}

	return accountsWithMissingPermissionSet, nil
}

func RemoveAccountPrefix(prefix string, val string) string {
	return strings.TrimPrefix(val, prefix)
}

func InitManageSso(cfg aws.Config, identityStoreArn string) *ManageSso {
	payload := &ManageSso{
		awsAccountsByAlias: map[string]*orgTypes.Account{},
		awsAccountsById:    map[string]*orgTypes.Account{},
		awsSsoGroupById:    map[string]*identityStoreTypes.Group{},
		awsSsoGroupByName:  map[string]*identityStoreTypes.Group{},
	}

	orgClient := organizations.NewFromConfig(cfg)
	identityStoreClient := identitystore.NewFromConfig(cfg)

	awsAccounts := GetAccounts(orgClient)
	groups, err := GetGroups(identityStoreClient, identityStoreArn)
	if err != nil {
		log.Fatal(err)
	}

	for _, acc := range awsAccounts {
		newAcc := &orgTypes.Account{
			Arn:             acc.Arn,
			Email:           acc.Email,
			Id:              acc.Id,
			JoinedMethod:    acc.JoinedMethod,
			JoinedTimestamp: acc.JoinedTimestamp,
			Name:            acc.Name,
			Status:          acc.Status,
		}
		payload.AwsAccounts = append(payload.AwsAccounts, newAcc)
		payload.awsAccountsById[*acc.Id] = newAcc
		payload.awsAccountsByAlias[*acc.Name] = newAcc
	}

	for _, group := range groups {
		newGroup := &identityStoreTypes.Group{
			GroupId:         group.GroupId,
			IdentityStoreId: group.IdentityStoreId,
			Description:     group.Description,
			DisplayName:     group.DisplayName,
			ExternalIds:     group.ExternalIds,
		}
		payload.AwsSsoGroups = append(payload.AwsSsoGroups, newGroup)
		payload.awsSsoGroupById[*group.GroupId] = newGroup
		payload.awsSsoGroupByName[*group.DisplayName] = newGroup
	}

	return payload
}