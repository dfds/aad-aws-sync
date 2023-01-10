package main

import (
	"context"
	"fmt"
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

	// Capability PermissionSet
	accountsWithMissingPermissionSet, err := manageSso.GetAccountsMissingCapabilityPermissionSet(ssoClient, conf.Aws.SsoInstanceArn, conf.Aws.CapabilityPermissionSetArn, CAPABILITY_GROUP_PREFIX, conf.Aws.AccountNamePrefix)
	if err != nil {
		log.Fatal(err)
	}

	for _, resp := range accountsWithMissingPermissionSet {
		fmt.Printf("Assigning Capability access to group %s for account %s\n", *resp.Group.DisplayName, *resp.Account.Name)
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

	// CapabilityLog PermissionSet
	acc := manageSso.GetAccountByName(conf.Aws.CapabilityLogsAwsAccountAlias)
	if acc == nil {
		log.Fatal("Unable to find CapabilityLogsAwsAccount by alias")
	}

	resp, err := manageSso.GetGroupsNotAssignedToAccountWithPermissionSet(ssoClient, conf.Aws.SsoInstanceArn, conf.Aws.CapabilityLogsPermissionSetArn, *acc.Id, CAPABILITY_GROUP_PREFIX)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Currently not assigned access:")
	for _, grp := range resp.GroupsNotAssigned {
		fmt.Printf("  %s\n", *grp.DisplayName)
	}

	fmt.Println("Currently assigned access:")
	for _, grp := range resp.GroupsAssigned {
		fmt.Printf("  %s\n", *grp.DisplayName)
	}

	for _, grp := range resp.GroupsNotAssigned {
		fmt.Printf("Assigning CapabilityLog access to %s\n", *grp.DisplayName)
		_, err := ssoClient.CreateAccountAssignment(context.TODO(), &ssoadmin.CreateAccountAssignmentInput{
			InstanceArn:      &conf.Aws.SsoInstanceArn,
			PermissionSetArn: &conf.Aws.CapabilityLogsPermissionSetArn,
			PrincipalId:      grp.GroupId,
			PrincipalType:    "GROUP",
			TargetId:         acc.Id,
			TargetType:       "AWS_ACCOUNT",
		})
		if err != nil {
			log.Fatal(err)
		}
	}

}
