package util

import (
	"encoding/json"
	"go.dfds.cloud/aad-aws-sync/aws"
	"log"
	"os"
)

type TestData struct {
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
