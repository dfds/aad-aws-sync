package handler

import (
	"context"
	"errors"
	"fmt"
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
	"log"
	"strings"
	"time"
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
			log.Printf("unable to assume role %s, %v", conf.Aws.AssumableRoles.SsoManagementArn, err)
			return err
		}

		cfg, err = awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*assumedRole.Credentials.AccessKeyId, *assumedRole.Credentials.SecretAccessKey, *assumedRole.Credentials.SessionToken)), awsConfig.WithRegion(conf.Aws.SsoRegion))
		if err != nil {
			return errors.New(fmt.Sprintf("unable to load SDK config, %v", err))
		}
	}

	orgClient := organizations.NewFromConfig(cfg)

	accounts := aws.GetAccounts(orgClient)
	ssoRoleMappings := []aws.SsoRoleMapping{}

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

	resp := aws.GetSsoRoles(ssoRoleMappings, conf.Aws.AssumableRoles.CapabilityAccountRoleName)
	//for _, acc := range resp {
	//	fmt.Printf("AWS Account: %s\nSSO role name: %s\nSSO role arn: %s\n", acc.AccountAlias, acc.RoleName, acc.RoleArn)
	//}

	amResp, err := k8s.LoadAwsAuthMapRoles()
	if err != nil {
		log.Fatal(err)
	}

	for _, mapping := range amResp.Mappings {
		if mapping.ManagedByThis() {
			//fmt.Println(mapping)
			//currentTime := time.Now()
			//mapping.LastUpdated = currentTime.Format(TIME_FORMAT)
		}
	}

	for _, acc := range resp {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", AwsToKubernetesName))
			return nil
		default:
		}

		mapping := amResp.GetMappingByArn(fmt.Sprintf("arn:aws:iam::%s:role/%s", acc.AccountId, acc.RoleName))
		currentTime := time.Now()

		if mapping == nil {
			fmt.Printf("No mapping for %s, creating.\n", acc.AccountAlias)
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
			//fmt.Printf("Mapping for %s discovered. Checking if update is needed\n", acc.AccountAlias)
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
				fmt.Printf("Config mismatch for %s detected, updating entry\n", acc.AccountAlias)

				mapping.Username = fmt.Sprintf("%s:sso-{{SessionName}}", acc.RootId)
				mapping.Groups = []string{"DFDS-ReadOnly", acc.RootId}
				mapping.LastUpdated = currentTime.Format(TIME_FORMAT)
			}
		}
	}

	payload, err := yaml.Marshal(&amResp.Mappings)
	if err != nil {
		log.Fatal(err)
	}

	amResp.ConfigMap.Data["mapRoles"] = string(payload)

	err = k8s.UpdateAwsAuthMapRoles(amResp.ConfigMap)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}
