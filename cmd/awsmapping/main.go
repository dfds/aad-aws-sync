package main

import (
	"context"
	dconfig "go.dfds.cloud/aad-aws-sync/internal/config"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"

	"log"

	"go.dfds.cloud/aad-aws-sync/internal/aws"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	conf, err := dconfig.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-1"), config.WithHTTPClient(aws.CreateHttpClientWithoutKeepAlive()))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	ssoClient := ssoadmin.NewFromConfig(cfg)

	manageSso := aws.InitManageSso(cfg, conf.Aws.IdentityStoreArn)

	accountsWithMissingPermissionSet, err := manageSso.GetAccountsMissingCapabilityPermissionSet(ssoClient, conf.Aws.SsoInstanceArn, conf.Aws.CapabilityPermissionSetArn, CAPABILITY_GROUP_PREFIX, conf.Aws.AccountNamePrefix)
	if err != nil {
		log.Fatal(err)
	}

	for _, resp := range accountsWithMissingPermissionSet {
		_, err := ssoClient.CreateAccountAssignment(context.TODO(), &ssoadmin.CreateAccountAssignmentInput{
			InstanceArn:      &conf.Aws.SsoInstanceArn,
			PermissionSetArn: &conf.Aws.CapabilityPermissionSetArn,
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
