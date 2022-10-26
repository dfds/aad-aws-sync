package util

import (
	"encoding/json"
	"go.dfds.cloud/aad-aws-sync/aws"
	"log"
	"os"
)

type TestData struct {
	Azure struct {
		TenantId     string `json:"tenantId"`
		ClientId     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	} `json:"azure"`
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
