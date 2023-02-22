package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	daws "github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.dfds.cloud/aad-aws-sync/internal/aws"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/k8s"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const AwsToKubernetesName = "awsToK8s"

func Aws2K8sHandler(ctx context.Context) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	var cfg daws.Config

	cfg, err = awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(conf.Aws.SsoRegion), awsConfig.WithHTTPClient(aws.CreateHttpClientWithoutKeepAlive()))
	if err != nil {
		return errors.New(fmt.Sprintf("unable to load SDK config, %v", err))
	}

	if conf.Aws.AssumableRoles.SsoManagementArn != "" {
		stsClient := sts.NewFromConfig(cfg)
		roleSessionName := fmt.Sprintf("aad-aws-sync-%s", AwsMappingName)

		assumedRole, err := stsClient.AssumeRole(context.TODO(), &sts.AssumeRoleInput{RoleArn: &conf.Aws.AssumableRoles.SsoManagementArn, RoleSessionName: &roleSessionName})
		if err != nil {
			util.Logger.Info(fmt.Sprintf("unable to assume role %s, %v", conf.Aws.AssumableRoles.SsoManagementArn, err), zap.String("jobName", AwsToKubernetesName))
			return err
		}

		cfg, err = awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*assumedRole.Credentials.AccessKeyId, *assumedRole.Credentials.SecretAccessKey, *assumedRole.Credentials.SessionToken)), awsConfig.WithRegion(conf.Aws.SsoRegion))
		if err != nil {
			return errors.New(fmt.Sprintf("unable to load SDK config, %v", err))
		}
	}

	orgClient := organizations.NewFromConfig(cfg)

	// Get all AWS accounts
	accounts, err := aws.GetAccounts(orgClient)
	if err != nil {
		return err
	}
	ssoRoleMappings := []aws.SsoRoleMapping{}

	// Put AWS accounts in a useful format
	for _, acc := range accounts {
		ssoRoleMapping := aws.SsoRoleMapping{
			AccountAlias: *acc.Name,
			AccountId:    *acc.Id,
			RoleName:     "",
			RoleArn:      "",
			RootId:       strings.Replace(*acc.Name, "dfds-", "", 1),
		}
		ssoRoleMappings = append(ssoRoleMappings, ssoRoleMapping)
	}

	// Populate rolename rolearn from api+config
	resp, err := aws.GetSsoRoles(ssoRoleMappings, conf.Aws.AssumableRoles.CapabilityAccountRoleName)
	if err != nil {
		return err
	}

	amResp, err := k8s.LoadAwsAuthMapRoles()
	if err != nil {
		return err
	}

	// Loop through ConfigMap entries, check if an entry exists where the equivalent AWS role doesn't. If that's the case, remove the entry from aws-auth ConfigMap
	for x := 0; x < len(amResp.Mappings); x++ {
		if amResp.Mappings[x].ManagedByThis() {
			// Check for mapping RoleArn exists/matches resp RoleArn
			match := false

			for _, r := range resp {
				arnSlice := strings.Split(r.RoleArn, "/")
				arnTrimmed := arnSlice[0] + "/" + arnSlice[len(arnSlice)-1]
				if amResp.Mappings[x].RoleARN == arnTrimmed {
					match = true
				}
			}

			if !match {
				util.Logger.Info(fmt.Sprintf("Role no longer found. Removing %s", amResp.Mappings[x].RoleARN), zap.String("jobName", AwsToKubernetesName))
				amResp.Mappings = removeArrayItem(amResp.Mappings, x)
			}
		}
	}

	// Loop through AWS account info where a capability access role is found
	for _, acc := range resp {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", AwsToKubernetesName))
			return nil
		default:
		}

		mapping := amResp.GetMappingByArn(fmt.Sprintf("arn:aws:iam::%s:role/%s", acc.AccountId, acc.RoleName))
		currentTime := time.Now()

		// If no config-map entry for aws acc with role
		if mapping == nil {
			util.Logger.Info(fmt.Sprintf("No mapping for %s, creating.\n", acc.AccountAlias), zap.String("jobName", AwsToKubernetesName))
			roleMapping := &k8s.RoleMapping{
				RoleARN:     fmt.Sprintf("arn:aws:iam::%s:role/%s", acc.AccountId, acc.RoleName),
				ManagedBy:   "aad-aws-sync",
				LastUpdated: currentTime.Format(TIME_FORMAT),
				CreatedAt:   currentTime.Format(TIME_FORMAT),
				Username:    fmt.Sprintf("%s:sso-{{SessionName}}", acc.RootId),
				Groups:      []string{"DFDS-ReadOnly", acc.RootId},
			}
			amResp.Mappings = append(amResp.Mappings, roleMapping)
		} else {
			configMismatch := false

			if mapping.Username != fmt.Sprintf("%s:sso-{{SessionName}}", acc.RootId) {
				configMismatch = true
			}

			if !mapping.ContainsGroup("DFDS-ReadOnly") {
				configMismatch = true
			}

			if !mapping.ContainsGroup(acc.RootId) {
				configMismatch = true
			}

			if configMismatch {
				util.Logger.Info(fmt.Sprintf("Config mismatch for %s detected, updating entry\n", acc.AccountAlias), zap.String("jobName", AwsToKubernetesName))

				mapping.Username = fmt.Sprintf("%s:sso-{{SessionName}}", acc.RootId)
				mapping.Groups = []string{"DFDS-ReadOnly", acc.RootId}
				mapping.LastUpdated = currentTime.Format(TIME_FORMAT)
			}
		}
	}

	payload, err := yaml.Marshal(&amResp.Mappings)
	if err != nil {
		return err
	}

	amResp.ConfigMap.Data["mapRoles"] = string(payload)

	err = k8s.UpdateAwsAuthMapRoles(amResp.ConfigMap)
	if err != nil {
		return err
	}
	return nil
}

func removeArrayItem(s []*k8s.RoleMapping, i int) []*k8s.RoleMapping {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}
