package main

import (
	"fmt"
	"log"

	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	testData := util.LoadTestData()

	azClient := azure.NewAzureClient(azure.Config{
		TenantId:     testData.Azure.TenantId,
		ClientId:     testData.Azure.ClientId,
		ClientSecret: testData.Azure.ClientSecret,
	})

	appRoles, err := azClient.GetApplicationRoles(testData.Azure.ApplicationId)
	if err != nil {
		log.Fatal(err)
	}

	appRoleId, err := appRoles.GetRoleId("User")
	if err != nil {
		log.Fatal(err)
	}

	appAssignments, err := azClient.GetAssignmentsForApplication(testData.Azure.ApplicationObjectId)
	if err != nil {
		log.Fatal(err)
	}

	groups, err := azClient.GetGroups(CAPABILITY_GROUP_PREFIX)
	if err != nil {
		log.Fatal(err)
	}

	for _, group := range groups.Value {
		fmt.Println(group.DisplayName)

		// If group is not already assigned to enterprise application, assign them.
		if !appAssignments.ContainsGroup(group.DisplayName) {
			fmt.Printf("Group %s has not been assigned to application yet, assigning.\n", group.DisplayName)
			_, err := azClient.AssignGroupToApplication(testData.Azure.ApplicationObjectId, group.ID, appRoleId)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

}
