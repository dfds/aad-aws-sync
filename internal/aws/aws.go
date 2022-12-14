package aws

import (
	"context"
	"fmt"
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
	"golang.org/x/sync/semaphore"
	"log"
	"net/http"
	"strings"
	"sync"
)

type SsoRoleMapping struct {
	AccountAlias string
	AccountId    string
	RoleName     string
	RoleArn      string
	RootId       string
}

func GetAccounts(client *organizations.Client) []orgTypes.Account {
	var maxResults int32 = 20
	var accounts []orgTypes.Account
	resps := organizations.NewListAccountsPaginator(client, &organizations.ListAccountsInput{MaxResults: &maxResults})
	for resps.HasMorePages() { // Due to the limit of only 20 accounts per query and wanting to avoid getting hit by a rate limit, this will take a while if you have a decent amount of AWS accounts
		page, err := resps.NextPage(context.TODO())
		if err != nil {
			log.Fatal(err)
		}

		accounts = append(accounts, page.Accounts...)
	}

	return accounts
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

func GetSsoRoles(accounts []SsoRoleMapping, roleName string) map[string]SsoRoleMapping {
	payload := make(map[string]SsoRoleMapping)
	rolePathPrefix := "/aws-reserved"
	roleNamePrefix := "AWSReservedSSO_CapabilityAccess"
	var maxConcurrentOps int64 = 30

	var waitGroup sync.WaitGroup
	sem := semaphore.NewWeighted(maxConcurrentOps)
	ctx := context.TODO()

	for _, acc := range accounts {

		waitGroup.Add(1)
		acc := acc
		go func() {
			sem.Acquire(ctx, 1)
			defer sem.Release(1)
			defer waitGroup.Done()

			roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", acc.AccountId, roleName)
			cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-1"), config.WithHTTPClient(CreateHttpClientWithoutKeepAlive()))
			if err != nil {
				log.Fatalf("unable to load SDK config, %v", err)
			}

			stsClient := sts.NewFromConfig(cfg)
			roleSessionName := "aad-aws-sync"
			assumedRole, err := stsClient.AssumeRole(context.TODO(), &sts.AssumeRoleInput{RoleArn: &roleArn, RoleSessionName: &roleSessionName})
			if err != nil {
				log.Printf("unable to assume role %s\nAccount %s (%s) is likely missing the IAM role 'SSO_list_iam_roles' or it is misconfigured, skipping account.\nOriginal error: %v\n\n", roleArn, acc.AccountAlias, acc.AccountId, err)
				return
			}

			assumedCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*assumedRole.Credentials.AccessKeyId, *assumedRole.Credentials.SecretAccessKey, *assumedRole.Credentials.SessionToken)), config.WithRegion("eu-west-1"))
			if err != nil {
				log.Fatalf("unable to load SDK config, %v", err)
			}

			// get a new client using the config we just generated
			assumedClient := iam.NewFromConfig(assumedCfg)
			resp, err := assumedClient.ListRoles(context.TODO(), &iam.ListRolesInput{PathPrefix: &rolePathPrefix})
			if err != nil {
				log.Fatalf("Unable to list IAM roles %v", err)
			}

			for _, role := range resp.Roles {
				if strings.Contains(*role.RoleName, roleNamePrefix) {
					acc.RoleName = *role.RoleName
					acc.RoleArn = *role.Arn
					payload[acc.AccountAlias] = acc
				}
			}
		}()
	}

	waitGroup.Wait()

	return payload
}

func CreateHttpClientWithoutKeepAlive() *awsHttp.BuildableClient {
	client := awsHttp.NewBuildableClient().WithTransportOptions(func(transport *http.Transport) {
		transport.DisableKeepAlives = true
	})

	return client
}
