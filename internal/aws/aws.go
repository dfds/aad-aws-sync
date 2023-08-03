package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	awsHttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identityTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

type SsoRoleMapping struct {
	AccountAlias string
	AccountId    string
	RoleName     string
	RoleArn      string
	RootId       string
}

type ScimClient struct {
	token    string
	endpoint string
	http     *http.Client
}

func CreateScimClient(endpoint string, token string) *ScimClient {
	sc := &ScimClient{
		token:    token,
		endpoint: endpoint,
	}
	httpClient := http.DefaultClient
	sc.http = httpClient

	return sc
}

func (c *ScimClient) prepareHttpRequest(h *http.Request) error {
	h.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	h.Header.Set("User-Agent", "aad-aws-sync - github.com/dfds/aad-aws-sync")

	return nil
}

func (c *ScimClient) GetUserViaExternalId(id string) (*ScimGetUserResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/scim/v2/Users", c.endpoint), nil)
	if err != nil {
		return nil, err
	}

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("filter", fmt.Sprintf("externalId eq \"%s\"", id))
	req.URL.RawQuery = urlQueryValues.Encode()

	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *ScimGetUsersResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	if payload.TotalResults == 1 {
		return payload.Resources[0], nil
	}

	return nil, errors.New("response didn't return 1 exact match")
}

func (c *ScimClient) GetGroupViaDisplayName(name string) (*ScimGetGroupResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/scim/v2/Groups", c.endpoint), nil)
	if err != nil {
		return nil, err
	}

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("filter", fmt.Sprintf("displayName eq \"%s\"", name))
	req.URL.RawQuery = urlQueryValues.Encode()

	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *ScimGetGroupsResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	if payload.TotalResults == 1 {
		return payload.Resources[0], nil
	}

	return nil, errors.New("response didn't return 1 exact match")
}

func (c *ScimClient) CreateGroup(data ScimCreateGroupRequest) error {
	reqPayload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/scim/v2/Groups", c.endpoint), bytes.NewBuffer(reqPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		if resp.StatusCode == 409 {
			return nil
		}
		return errors.New(fmt.Sprintf("Received unexpected status code response, %d", resp.StatusCode))
	}

	return nil
}

func (c *ScimClient) RemoveUser(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/scim/v2/Users/%s", c.endpoint, id), nil)
	if err != nil {
		return err
	}

	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return errors.New(fmt.Sprintf("Received unexpected status code response, %d", resp.StatusCode))
	}

	return nil
}

func (c *ScimClient) RemoveGroup(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/scim/v2/Groups/%s", c.endpoint, id), nil)
	if err != nil {
		return err
	}

	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return errors.New(fmt.Sprintf("Received unexpected status code response, %d", resp.StatusCode))
	}

	return nil
}

func (c *ScimClient) CreateUser(data ScimCreateUserRequest) error {
	reqPayload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/scim/v2/Users", c.endpoint), bytes.NewBuffer(reqPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		if resp.StatusCode == 409 {
			return nil
		}
		return errors.New(fmt.Sprintf("Received unexpected status code response, %d", resp.StatusCode))
	}

	return nil
}

func (c *ScimClient) PatchAddMembersToGroup(groupId string, members ...string) error {
	reqPatch := NewScimPatchAddMembersToGroupRequest(members...)
	reqPayload, err := json.Marshal(reqPatch)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/scim/v2/Groups/%s", c.endpoint, groupId), bytes.NewBuffer(reqPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return errors.New(fmt.Sprintf("Received unexpected status code response, %d", resp.StatusCode))
	}

	return nil
}

func (c *ScimClient) PatchRemoveMembersFromGroup(groupId string, members ...string) error {
	reqPatch := NewScimPatchRemoveMembersToGroupRequest(members...)
	reqPayload, err := json.Marshal(reqPatch)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/scim/v2/Groups/%s", c.endpoint, groupId), bytes.NewBuffer(reqPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return errors.New(fmt.Sprintf("Received unexpected status code response, %d", resp.StatusCode))
	}

	return nil
}

func GetAccounts(client *organizations.Client, parentId string) ([]orgTypes.Account, error) {
	var maxResults int32 = 20
	var accounts []orgTypes.Account
	resps := organizations.NewListAccountsForParentPaginator(client, &organizations.ListAccountsForParentInput{MaxResults: &maxResults, ParentId: &parentId})
	for resps.HasMorePages() { // Due to the limit of only 20 accounts per query and wanting to avoid getting hit by a rate limit, this will take a while if you have a decent amount of AWS accounts
		page, err := resps.NextPage(context.TODO())
		if err != nil {
			return accounts, err
		}

		accounts = append(accounts, page.Accounts...)
	}

	return accounts, nil
}

func GetGroups(client *identitystore.Client, identityStoreArn string) ([]identityTypes.Group, error) {
	var maxResults int32 = 100
	var payload []identityTypes.Group
	resps := identitystore.NewListGroupsPaginator(client, &identitystore.ListGroupsInput{MaxResults: &maxResults, IdentityStoreId: &identityStoreArn})
	for resps.HasMorePages() {
		page, err := resps.NextPage(context.TODO())
		if err != nil {
			return payload, err
		}

		payload = append(payload, page.Groups...)
	}

	return payload, nil
}

func GetGroupMemberships(client *identitystore.Client, identityStoreArn string, groupId *string) ([]identityTypes.GroupMembership, error) {
	var maxResults int32 = 100
	var payload []identityTypes.GroupMembership
	resps := identitystore.NewListGroupMembershipsPaginator(client, &identitystore.ListGroupMembershipsInput{MaxResults: &maxResults, IdentityStoreId: &identityStoreArn, GroupId: groupId})
	for resps.HasMorePages() {
		page, err := resps.NextPage(context.TODO())
		if err != nil {
			return payload, err
		}

		payload = append(payload, page.GroupMemberships...)
	}

	return payload, nil
}

func GetPermissionSets(client *ssoadmin.Client, instanceArn string) ([]string, error) {
	var maxResults int32 = 100
	var payload []string
	resps := ssoadmin.NewListPermissionSetsPaginator(client, &ssoadmin.ListPermissionSetsInput{MaxResults: &maxResults, InstanceArn: &instanceArn})
	for resps.HasMorePages() {
		page, err := resps.NextPage(context.TODO())
		if err != nil {
			return payload, err
		}

		payload = append(payload, page.PermissionSets...)
	}

	return payload, nil
}

func GetAccountsWithProvisionedPermissionSet(client *ssoadmin.Client, instanceArn string, permissionSetArn string) ([]string, error) {
	var maxResults int32 = 100
	var payload []string
	resps := ssoadmin.NewListAccountsForProvisionedPermissionSetPaginator(client, &ssoadmin.ListAccountsForProvisionedPermissionSetInput{MaxResults: &maxResults, InstanceArn: &instanceArn, PermissionSetArn: &permissionSetArn})
	for resps.HasMorePages() {
		page, err := resps.NextPage(context.TODO())
		if err != nil {
			return payload, err
		}

		payload = append(payload, page.AccountIds...)
	}

	return payload, nil
}

func GetAssignedForPermissionSetInAccount(client *ssoadmin.Client, ssoInstanceArn string, permissionSetArn string, accountId string) ([]types.AccountAssignment, error) {
	var maxResults int32 = 100
	var payload []types.AccountAssignment
	resps := ssoadmin.NewListAccountAssignmentsPaginator(client, &ssoadmin.ListAccountAssignmentsInput{
		AccountId:        &accountId,
		InstanceArn:      &ssoInstanceArn,
		PermissionSetArn: &permissionSetArn,
		MaxResults:       &maxResults,
	})

	for resps.HasMorePages() {
		page, err := resps.NextPage(context.TODO())
		if err != nil {
			return payload, err
		}

		payload = append(payload, page.AccountAssignments...)
	}

	return payload, nil
}

func GetSsoRoles(accounts []SsoRoleMapping, roleName string) (map[string]SsoRoleMapping, error) {
	payload := make(map[string]SsoRoleMapping)
	rolePathPrefix := "/aws-reserved"
	roleNamePrefix := "AWSReservedSSO_CapabilityAccess"
	var maxConcurrentOps int64 = 30

	var waitGroup sync.WaitGroup
	payloadMutex := &sync.Mutex{}
	sem := semaphore.NewWeighted(maxConcurrentOps)
	ctx := context.TODO()

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-1"), config.WithHTTPClient(CreateHttpClientWithoutKeepAlive()))
	if err != nil {
		return payload, err
	}

	for _, acc := range accounts {
		waitGroup.Add(1)
		acc := acc
		go func() {
			sem.Acquire(ctx, 1)
			defer sem.Release(1)
			defer waitGroup.Done()

			roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", acc.AccountId, roleName)

			stsClient := sts.NewFromConfig(cfg)
			roleSessionName := "aad-aws-sync"
			assumedRole, err := stsClient.AssumeRole(context.TODO(), &sts.AssumeRoleInput{RoleArn: &roleArn, RoleSessionName: &roleSessionName})
			if err != nil {
				util.Logger.Debug(fmt.Sprintf("unable to assume role %s. Account %s (%s) is likely missing the IAM role 'sso-reader' or it is misconfigured, skipping account", roleArn, acc.AccountAlias, acc.AccountId), zap.Error(err))
				return
			}

			assumedCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*assumedRole.Credentials.AccessKeyId, *assumedRole.Credentials.SecretAccessKey, *assumedRole.Credentials.SessionToken)), config.WithRegion("eu-west-1"))
			if err != nil {
				util.Logger.Error(fmt.Sprintf("unable to load SDK config, %v", err))
				return
			}

			// get a new client using the config we just generated
			assumedClient := iam.NewFromConfig(assumedCfg)
			resp, err := assumedClient.ListRoles(context.TODO(), &iam.ListRolesInput{PathPrefix: &rolePathPrefix})
			if err != nil {
				util.Logger.Error(fmt.Sprintf("Unable to list IAM roles %v", err))
				return
			}

			for _, role := range resp.Roles {
				if strings.Contains(*role.RoleName, roleNamePrefix) {
					acc.RoleName = *role.RoleName
					acc.RoleArn = *role.Arn
					payloadMutex.Lock()
					payload[acc.AccountAlias] = acc
					payloadMutex.Unlock()
				}
			}
		}()
	}

	waitGroup.Wait()

	return payload, nil
}

func CreateHttpClientWithoutKeepAlive() *awsHttp.BuildableClient {
	client := awsHttp.NewBuildableClient().WithTransportOptions(func(transport *http.Transport) {
		transport.DisableKeepAlives = true
	})

	return client
}
