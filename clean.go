package main

import (
	"fmt"
	"go.dfds.cloud/aad-aws-sync/azure"
	"go.dfds.cloud/aad-aws-sync/capsvc"
	"go.dfds.cloud/aad-aws-sync/util"
	"log"
)

func main() {
	testData := util.LoadTestData()
	capabilitiesByRootId := make(map[string]*capsvc.GetCapabilitiesResponseContextCapability)
	groupsInAzure := make(map[string]*azure.Group)

	azClient := azure.NewAzureClient(azure.Config{
		TenantId:     testData.Azure.TenantId,
		ClientId:     testData.Azure.ClientId,
		ClientSecret: testData.Azure.ClientSecret,
	})

	capClient := capsvc.NewCapSvcClient(testData.CapSvc.Host)

	capabilities, err := capClient.GetCapabilities()
	if err != nil {
		log.Fatal(err)
	}

	for _, capability := range capabilities.Items {
		_, err := capability.GetContext()
		if err == nil {
			capabilitiesByRootId[capability.RootID] = capability
		}
	}

	aUnits, err := azClient.GetAdministrativeUnits()
	if err != nil {
		log.Fatal(err)
	}

	aUnit := aUnits.GetUnit("Team - Cloud Engineering - Self service")
	if aUnit == nil {
		log.Fatal("Unable to find administrative unit")
	}

	aUnitMembers, err := azClient.GetAdministrativeUnitMembers(aUnit.ID)
	if err != nil {
		log.Fatal(err)
	}

	for _, member := range aUnitMembers.Value {
		group := &azure.Group{
			DisplayName: member.DisplayName,
			ID:          member.ID,
			Members:     []*azure.Member{},
		}
		groupMembers, err := azClient.GetGroupMembers(member.ID)
		if err != nil {
			log.Fatal(err)
		}

		for _, groupMember := range groupMembers.Value {
			group.Members = append(group.Members, &azure.Member{
				ID:                groupMember.ID,
				DisplayName:       groupMember.DisplayName,
				UserPrincipalName: groupMember.UserPrincipalName,
			})
		}

		groupsInAzure[group.DisplayName] = group
	}

	// Remove group assignments for enterprise application
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

		assignment := appAssignments.GetAssignmentByGroupName(group.DisplayName)
		if assignment == nil {
			log.Fatal(err)
		}

		err = azClient.UnassignGroupFromApplication(group.ID, assignment.ID)
		if err != nil {
			log.Fatal(err)
		}

	}

	for rootId, _ := range capabilitiesByRootId {
		azureGroupName := fmt.Sprintf("%s %s", CAPABILITY_GROUP_PREFIX, rootId)
		var azureGroup *azure.Group

		// Check if Capability has a group in Azure AD, if it doesn't create it
		if resp, ok := groupsInAzure[azureGroupName]; !ok {
			continue
		} else {
			azureGroup = resp
			fmt.Printf("Deleting group %s\n", azureGroup.DisplayName)
			err = azClient.DeleteAdministrativeUnitGroup(aUnit.ID, azureGroup.ID)
			if err != nil {
				log.Fatal(err)
			}
			continue
		}

	}

}
