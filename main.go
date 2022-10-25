package main

import (
	"fmt"
	"go.dfds.cloud/aad-aws-sync/aws"
	"go.dfds.cloud/aad-aws-sync/k8s"
	"go.dfds.cloud/aad-aws-sync/util"
)

func main() {
	testData := util.LoadTestData()

	resp := aws.GetSsoRoles(testData.AwsAccounts)
	for _, acc := range resp {
		fmt.Printf("AWS Account: %s\nSSO role name: %s\nSSO role arn: %s\n", acc.AccountAlias, acc.RoleName, acc.RoleArn)
	}

	//k8s.GenerateAwsAuthMapRolesObject(resp)
	k8s.LoadAwsAuthMap()
}
