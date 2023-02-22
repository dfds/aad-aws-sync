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
			util.Logger.Debug(fmt.Sprintf("unable to assume role %s, %v", conf.Aws.AssumableRoles.SsoManagementArn, err))
			return err
		}

		cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*assumedRole.Credentials.AccessKeyId, *assumedRole.Credentials.SecretAccessKey, *assumedRole.Credentials.SessionToken)), config.WithRegion(conf.Aws.SsoRegion))
		if err != nil {
			return errors.New(fmt.Sprintf("unable to load SDK config, %v", err))
		}
	}

	ssoClient := ssoadmin.NewFromConfig(cfg)

	manageSso, err := aws.InitManageSso(cfg, conf.Aws.IdentityStoreArn)
	if err != nil {
		return err
	}

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
		util.Logger.Info(fmt.Sprintf("Assigning Capability access to group %s for account %s\n", *resp.Group.DisplayName, *resp.Account.Name), zap.String("jobName", AwsMappingName))
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
	err = addToSharedRole(manageSso, ssoClient, addToSharedRoleRequest{
		Name:                "CapabilityLog",
		AwsAccountNameAlias: conf.Aws.CapabilityLogsAwsAccountAlias,
		PermissionSetArn:    conf.Aws.CapabilityLogsPermissionSetArn,
		SsoInstanceArn:      conf.Aws.SsoInstanceArn,
		ctx:                 ctx,
	})
	if err != nil {
		return err
	}

	// Shared ECR pull PermissionSet
	err = addToSharedRole(manageSso, ssoClient, addToSharedRoleRequest{
		Name:                "SharedECRPull",
		AwsAccountNameAlias: conf.Aws.SharedEcrPullAwsAccountAlias,
		PermissionSetArn:    conf.Aws.SharedEcrPullPermissionSetArn,
		SsoInstanceArn:      conf.Aws.SsoInstanceArn,
		ctx:                 ctx,
	})
	if err != nil {
		return err
	}

	return nil
}

func addToSharedRole(manageSso *aws.ManageSso, ssoClient *ssoadmin.Client, req addToSharedRoleRequest) error {
	acc := manageSso.GetAccountByName(req.AwsAccountNameAlias)
	if acc == nil {
		return errors.New(fmt.Sprintf("Unable to find AWS account by alias %s", req.AwsAccountNameAlias))
	}

	resp, err := manageSso.GetGroupsNotAssignedToAccountWithPermissionSet(ssoClient, req.SsoInstanceArn, req.PermissionSetArn, *acc.Id, CAPABILITY_GROUP_PREFIX)
	if err != nil {
		return err
	}

	notAssignedGroupNames := []string{}
	for _, grp := range resp.GroupsNotAssigned {
		notAssignedGroupNames = append(notAssignedGroupNames, *grp.DisplayName)
	}
	util.Logger.Info("Groups missing assigned access", zap.String("jobName", AwsMappingName), zap.Strings("groups", notAssignedGroupNames))

	assignedGroupNames := []string{}
	for _, grp := range resp.GroupsAssigned {
		assignedGroupNames = append(assignedGroupNames, *grp.DisplayName)
	}

	for _, grp := range resp.GroupsNotAssigned {
		select {
		case <-req.ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", AwsMappingName))
			return nil
		default:
		}

		util.Logger.Info(fmt.Sprintf("Assigning access to %s\n", *grp.DisplayName), zap.String("jobName", AwsMappingName), zap.String("permissionSet", req.Name))
		_, err := ssoClient.CreateAccountAssignment(context.TODO(), &ssoadmin.CreateAccountAssignmentInput{
			InstanceArn:      &req.SsoInstanceArn,
			PermissionSetArn: &req.PermissionSetArn,
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

type addToSharedRoleRequest struct {
	Name                string
	AwsAccountNameAlias string
	PermissionSetArn    string
	SsoInstanceArn      string
	ctx                 context.Context
}
