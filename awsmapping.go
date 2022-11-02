package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"go.dfds.cloud/aad-aws-sync/aws"
	"go.dfds.cloud/aad-aws-sync/util"
	"log"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	testData := util.LoadTestData()

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-1"), config.WithHTTPClient(aws.CreateHttpClientWithoutKeepAlive()))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	ssoClient := ssoadmin.NewFromConfig(cfg)

	manageSso := aws.InitManageSso(cfg, testData.Aws.IdentityStoreArn)

	accountsWithMissingPermissionSet, err := manageSso.GetAccountsMissingCapabilityPermissionSet(ssoClient, testData.Aws.SsoInstanceArn, testData.Aws.CapabilityPermissionSetArn, CAPABILITY_GROUP_PREFIX, testData.Aws.AccountNamePrefix)
	if err != nil {
		log.Fatal(err)
	}

	for _, resp := range accountsWithMissingPermissionSet {
		_, err := ssoClient.CreateAccountAssignment(context.TODO(), &ssoadmin.CreateAccountAssignmentInput{
			InstanceArn:      &testData.Aws.SsoInstanceArn,
			PermissionSetArn: &testData.Aws.CapabilityPermissionSetArn,
			PrincipalId:      resp.Group.GroupId,
			PrincipalType:    "GROUP",
			TargetId:         resp.Account.Id,
			TargetType:       "AWS_ACCOUNT",
		})
		if err != nil {
			log.Fatal(err)
		}
	}

}
