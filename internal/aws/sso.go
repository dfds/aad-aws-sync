package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identityStoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
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

func (m *ManageSso) GetGroupsNotAssignedToAccountWithPermissionSet(client *ssoadmin.Client, ssoInstanceArn string, permissionSetArn string, accountId string, groupPrefix string) (*GetGroupsNotAssignedToAccountWithPermissionSetResponse, error) {
	resp, err := GetAssignedForPermissionSetInAccount(client, ssoInstanceArn, permissionSetArn, accountId)
	if err != nil {
		return nil, err
	}
	var groupsCurrentlyAssignedByName = map[string]*identityStoreTypes.Group{}
	payload := &GetGroupsNotAssignedToAccountWithPermissionSetResponse{}

	for _, assignment := range resp {
		if assignment.PrincipalType == "GROUP" {
			group := m.GetGroupById(*assignment.PrincipalId)

			groupsCurrentlyAssignedByName[*group.DisplayName] = group
			payload.GroupsAssigned = append(payload.GroupsAssigned, group)
		}
	}

	for _, grp := range m.AwsSsoGroups {
		_, containsKey := groupsCurrentlyAssignedByName[*grp.DisplayName]
		if strings.Contains(*grp.DisplayName, groupPrefix) && !containsKey {
			payload.GroupsNotAssigned = append(payload.GroupsNotAssigned, grp)
		}
	}

	return payload, nil
}

func RemoveAccountPrefix(prefix string, val string) string {
	return strings.TrimPrefix(val, prefix)
}

func InitManageSso(cfg aws.Config, identityStoreArn string) (*ManageSso, error) {
	payload := &ManageSso{
		awsAccountsByAlias: map[string]*orgTypes.Account{},
		awsAccountsById:    map[string]*orgTypes.Account{},
		awsSsoGroupById:    map[string]*identityStoreTypes.Group{},
		awsSsoGroupByName:  map[string]*identityStoreTypes.Group{},
	}

	orgClient := organizations.NewFromConfig(cfg)
	identityStoreClient := identitystore.NewFromConfig(cfg)

	awsAccounts, err := GetAccounts(orgClient)
	if err != nil {
		return nil, err
	}
	groups, err := GetGroups(identityStoreClient, identityStoreArn)
	if err != nil {
		return nil, err
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

	return payload, nil
}
