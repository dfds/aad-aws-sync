package aws

import (
	"context"
	"fmt"
	awsHttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
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

func GetSsoRoles(accounts []SsoRoleMapping) map[string]SsoRoleMapping {
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

			roleArn := fmt.Sprintf("arn:aws:iam::%s:role/SSO_list_iam_roles", acc.AccountId)
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
