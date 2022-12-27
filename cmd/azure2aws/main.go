package main

import (
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"log"

	"go.dfds.cloud/aad-aws-sync/internal/azure"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	azClient := azure.NewAzureClient(azure.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
	})

	appRoles, err := azClient.GetApplicationRoles(conf.Azure.ApplicationId)
	if err != nil {
		log.Fatal(err)
	}

	appRoleId, err := appRoles.GetRoleId("User")
	if err != nil {
		log.Fatal(err)
	}

	appAssignments, err := azClient.GetAssignmentsForApplication(conf.Azure.ApplicationObjectId)
	if err != nil {
		log.Fatal(err)
	}

	groups, err := azClient.GetGroups(azure.AZURE_CAPABILITY_GROUP_PREFIX)
	if err != nil {
		log.Fatal(err)
	}

	for _, group := range groups.Value {
		fmt.Println(group.DisplayName)

		// If group is not already assigned to enterprise application, assign them.
		if !appAssignments.ContainsGroup(group.DisplayName) {
			fmt.Printf("Group %s has not been assigned to application yet, assigning.\n", group.DisplayName)
			_, err := azClient.AssignGroupToApplication(conf.Azure.ApplicationObjectId, group.ID, appRoleId)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

}
