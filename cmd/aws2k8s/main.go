package main

import (
	"fmt"
	"log"
	"time"

	"go.dfds.cloud/aad-aws-sync/internal/aws"
	"go.dfds.cloud/aad-aws-sync/internal/k8s"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"gopkg.in/yaml.v2"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	testData := util.LoadTestData()

	// TODO: Get AWS accounts via API instead of relying on local test data
	resp := aws.GetSsoRoles(testData.AwsAccounts, testData.AssumableRoles.CapabilityAccountRoleName)
	for _, acc := range resp {
		fmt.Printf("AWS Account: %s\nSSO role name: %s\nSSO role arn: %s\n", acc.AccountAlias, acc.RoleName, acc.RoleArn)
	}

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
}
