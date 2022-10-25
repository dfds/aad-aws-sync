package main

import (
	"fmt"
	"go.dfds.cloud/aad-aws-sync/aws"
	"go.dfds.cloud/aad-aws-sync/k8s"
	"go.dfds.cloud/aad-aws-sync/util"
	"gopkg.in/yaml.v2"
	"log"
	"time"
)

func main() {
	testData := util.LoadTestData()

	resp := aws.GetSsoRoles(testData.AwsAccounts)
	for _, acc := range resp {
		fmt.Printf("AWS Account: %s\nSSO role name: %s\nSSO role arn: %s\n", acc.AccountAlias, acc.RoleName, acc.RoleArn)
	}

	amResp, err := k8s.LoadAwsAuthMapRoles()

	for _, mapping := range amResp.Mappings {
		if mapping.ManagedByThis() {
			fmt.Println(mapping)

			currentTime := time.Now()
			mapping.LastUpdated = currentTime.Format("2006-01-02 15:04:05.999999999 -0700 MST")
		}
	}

	for _, acc := range resp {
		currentTime := time.Now()
		roleMapping := &k8s.RoleMapping{
			RoleARN:     fmt.Sprintf("arn:aws:iam::%s:role/%s", acc.AccountId, acc.RoleName),
			ManagedBy:   "aad-aws-sync",
			LastUpdated: currentTime.Format("2006-01-02 15:04:05.999999999 -0700 MST"),
			CreatedAt:   currentTime.Format("2006-01-02 15:04:05.999999999 -0700 MST"),
			Username:    fmt.Sprintf("%s:sso-{{SessionName}}", acc.RootId),
			Groups:      []string{"DFDS-ReadOnly", acc.RootId},
		}
		amResp.Mappings = append(amResp.Mappings, roleMapping)
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
