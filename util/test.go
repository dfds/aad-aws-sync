package util

import (
	"encoding/json"
	"go.dfds.cloud/aad-aws-sync/aws"
	"log"
	"os"
)

type TestData struct {
	AssumableRoles struct {
		SsoManagementArn          string `json:"ssoManagementArn"`
		CapabilityAccountRoleName string `json:"capabilityAccountRoleName"`
	} `json:"assumableRoles"`
	Azure struct {
		TenantId            string `json:"tenantId"`
		ClientId            string `json:"clientId"`
		ClientSecret        string `json:"clientSecret"`
		ApplicationId       string `json:"applicationId"`
		ApplicationObjectId string `json:"applicationObjectId"`
	} `json:"azure"`
	CapSvc struct {
		Host string `json:"host"`
	}
	AwsAccounts []aws.SsoRoleMapping `json:"awsAccounts"`
}

func LoadTestData() TestData {
	data, err := os.ReadFile("testdata.json")
	if err != nil {
		log.Fatal(err)
	}

	var payload TestData
	json.Unmarshal(data, &payload)
	if err != nil {
		log.Fatal(err)
	}

	return payload
}
