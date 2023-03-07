package aws

import (
	identityStoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	orgTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"time"
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

type GetGroupsNotAssignedToAccountWithPermissionSetResponse struct {
	GroupsNotAssigned []*identityStoreTypes.Group
	GroupsAssigned    []*identityStoreTypes.Group
}

type ScimGetUsersResponse struct {
	TotalResults int                    `json:"totalResults"`
	ItemsPerPage int                    `json:"itemsPerPage"`
	StartIndex   int                    `json:"startIndex"`
	Schemas      []string               `json:"schemas"`
	Resources    []*ScimGetUserResponse `json:"Resources"`
}

type ScimGetUserResponse struct {
	ID         string `json:"id"`
	ExternalID string `json:"externalId"`
	Meta       struct {
		ResourceType string    `json:"resourceType"`
		Created      time.Time `json:"created"`
		LastModified time.Time `json:"lastModified"`
	} `json:"meta"`
	Schemas  []string `json:"schemas"`
	UserName string   `json:"userName"`
	Name     struct {
		Formatted  string `json:"formatted"`
		FamilyName string `json:"familyName"`
		GivenName  string `json:"givenName"`
	} `json:"name"`
	DisplayName string `json:"displayName"`
	Title       string `json:"title"`
	Active      bool   `json:"active"`
	Emails      []struct {
		Value   string `json:"value"`
		Type    string `json:"type"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
	Addresses []struct {
		StreetAddress string `json:"streetAddress"`
		Locality      string `json:"locality"`
		PostalCode    string `json:"postalCode"`
		Country       string `json:"country"`
		Type          string `json:"type"`
		Primary       bool   `json:"primary"`
	} `json:"addresses"`
	UrnIetfParamsScimSchemasExtensionEnterprise21User struct {
		EmployeeNumber string `json:"employeeNumber"`
		Department     string `json:"department"`
		Manager        struct {
			Value string `json:"value"`
		} `json:"manager"`
	} `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.1:User"`
}

type ScimGetGroupsResponse struct {
	TotalResults int                     `json:"totalResults"`
	ItemsPerPage int                     `json:"itemsPerPage"`
	StartIndex   int                     `json:"startIndex"`
	Schemas      []string                `json:"schemas"`
	Resources    []*ScimGetGroupResponse `json:"Resources"`
}

type ScimCreateGroupRequest struct {
	DisplayName string `json:"displayName"`
	Externalid  string `json:"externalid"`
}

type ScimCreateUserRequest struct {
	UserName    string `json:"userName"`
	ExternalID  string `json:"externalId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
	Name        struct {
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
}

type ScimGetGroupResponse struct {
	ID         string `json:"id"`
	ExternalID string `json:"externalId"`
	Meta       struct {
		ResourceType string    `json:"resourceType"`
		Created      time.Time `json:"created"`
		LastModified time.Time `json:"lastModified"`
	} `json:"meta"`
	Schemas     []string      `json:"schemas"`
	DisplayName string        `json:"displayName"`
	Members     []interface{} `json:"members"`
}

type ScimPatchMembersToGroupRequest struct {
	Schemas    []string                                  `json:"schemas"`
	Operations []ScimPatchMembersToGroupOperationRequest `json:"Operations"`
}

type ScimPatchMembersToGroupOperationRequest struct {
	Op    string                                         `json:"op"`
	Path  string                                         `json:"path"`
	Value []ScimPatchMembersToGroupOperationValueRequest `json:"value"`
}

type ScimPatchMembersToGroupOperationValueRequest struct {
	Value string `json:"value"`
}

func NewScimPatchAddMembersToGroupRequest(members ...string) ScimPatchMembersToGroupRequest {
	payload := ScimPatchMembersToGroupRequest{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []ScimPatchMembersToGroupOperationRequest{
			{
				Op:    "add",
				Path:  "members",
				Value: []ScimPatchMembersToGroupOperationValueRequest{},
			},
		},
	}

	for _, member := range members {
		payload.Operations[0].Value = append(payload.Operations[0].Value, ScimPatchMembersToGroupOperationValueRequest{Value: member})
	}

	return payload
}

func NewScimPatchRemoveMembersToGroupRequest(members ...string) ScimPatchMembersToGroupRequest {
	payload := ScimPatchMembersToGroupRequest{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []ScimPatchMembersToGroupOperationRequest{
			{
				Op:    "remove",
				Path:  "members",
				Value: []ScimPatchMembersToGroupOperationValueRequest{},
			},
		},
	}

	for _, member := range members {
		payload.Operations[0].Value = append(payload.Operations[0].Value, ScimPatchMembersToGroupOperationValueRequest{Value: member})
	}

	return payload
}
