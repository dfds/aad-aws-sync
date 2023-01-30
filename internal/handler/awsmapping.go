package handler

import (
	"context"
	"errors"
	"fmt"
	daws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.dfds.cloud/aad-aws-sync/internal/aws"
	dconfig "go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"log"
)

const AwsMappingName = "awsMapping"

func AwsMappingHandler(ctx context.Context) error {
	conf, err := dconfig.LoadConfig()
	if err != nil {
		return err
	}

	var cfg daws.Config

	cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(conf.Aws.SsoRegion), config.WithHTTPClient(aws.CreateHttpClientWithoutKeepAlive()))
	if err != nil {
		return errors.New(fmt.Sprintf("unable to load SDK config, %v", err))
	}

	if conf.Aws.AssumableRoles.SsoManagementArn != "" {
		stsClient := sts.NewFromConfig(cfg)
		roleSessionName := fmt.Sprintf("aad-aws-sync-%s", AwsMappingName)

		assumedRole, err := stsClient.AssumeRole(context.TODO(), &sts.AssumeRoleInput{RoleArn: &conf.Aws.AssumableRoles.SsoManagementArn, RoleSessionName: &roleSessionName})
		if err != nil {
			log.Printf("unable to assume role %s, %v", conf.Aws.AssumableRoles.SsoManagementArn, err)
			return err
		}

		cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*assumedRole.Credentials.AccessKeyId, *assumedRole.Credentials.SecretAccessKey, *assumedRole.Credentials.SessionToken)), config.WithRegion(conf.Aws.SsoRegion))
		if err != nil {
			return errors.New(fmt.Sprintf("unable to load SDK config, %v", err))
		}
	}

	ssoClient := ssoadmin.NewFromConfig(cfg)

	manageSso := aws.InitManageSso(cfg, conf.Aws.IdentityStoreArn)

	// Capability PermissionSet
	accountsWithMissingPermissionSet, err := manageSso.GetAccountsMissingCapabilityPermissionSet(ssoClient, conf.Aws.SsoInstanceArn, conf.Aws.CapabilityPermissionSetArn, CAPABILITY_GROUP_PREFIX, conf.Aws.AccountNamePrefix)
	if err != nil {
		return err
	}

	for _, resp := range accountsWithMissingPermissionSet {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", AwsMappingName))
			return nil
		default:
		}
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
			return err
		}
	}

	// CapabilityLog PermissionSet
	acc := manageSso.GetAccountByName(conf.Aws.CapabilityLogsAwsAccountAlias)
	if acc == nil {
		log.Fatal("Unable to find CapabilityLogsAwsAccount by alias")
	}

	resp, err := manageSso.GetGroupsNotAssignedToAccountWithPermissionSet(ssoClient, conf.Aws.SsoInstanceArn, conf.Aws.CapabilityLogsPermissionSetArn, *acc.Id, CAPABILITY_GROUP_PREFIX)
	if err != nil {
		return err
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
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", AwsMappingName))
			return nil
		default:
		}

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
			return err
		}
	}

	return nil
}
