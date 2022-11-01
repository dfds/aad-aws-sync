package main

import (
	"fmt"
	"go.dfds.cloud/aad-aws-sync/aws"
	"go.dfds.cloud/aad-aws-sync/azure"
	"go.dfds.cloud/aad-aws-sync/capsvc"
	"go.dfds.cloud/aad-aws-sync/k8s"
	"go.dfds.cloud/aad-aws-sync/util"
	"gopkg.in/yaml.v2"
	"log"
	"time"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	//SyncCapSvcToAzure()
	SyncAzureToAws()
	//SyncAwsToK8s()
}

func SyncCapSvcToAzure() {
	testData := util.LoadTestData()
	groupsInAzure := make(map[string]*azure.Group)
	capabilitiesByRootId := make(map[string]*capsvc.GetCapabilitiesResponseContextCapability)
	client := capsvc.NewCapSvcClient(testData.CapSvc.Host)

	capabilities, err := client.GetCapabilities()
	if err != nil {
		log.Fatal(err)
	}

	azureClient := azure.NewAzureClient(azure.Config{
		TenantId:     testData.Azure.TenantId,
		ClientId:     testData.Azure.ClientId,
		ClientSecret: testData.Azure.ClientSecret,
	})

	aUnits, err := azureClient.GetAdministrativeUnits()
	if err != nil {
		log.Fatal(err)
	}

	aUnit := aUnits.GetUnit("Team - Cloud Engineering - Self service")
	if aUnit == nil {
		log.Fatal("Unable to find administrative unit")
	}

	aUnitMembers, err := azureClient.GetAdministrativeUnitMembers(aUnit.ID)
	if err != nil {
		log.Fatal(err)
	}

	for _, capability := range capabilities.Items {
		_, err := capability.GetContext()
		if err == nil {
			capabilitiesByRootId[capability.RootID] = capability
		}
	}

	for _, member := range aUnitMembers.Value {
		group := &azure.Group{
			DisplayName: member.DisplayName,
			ID:          member.ID,
			Members:     []*azure.Member{},
		}
		groupMembers, err := azureClient.GetGroupMembers(member.ID)
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

	for rootId, capability := range capabilitiesByRootId {
		azureGroupName := fmt.Sprintf("%s %s", CAPABILITY_GROUP_PREFIX, rootId)
		var azureGroup *azure.Group

		// Check if Capability has a group in Azure AD, if it doesn't create it
		if resp, ok := groupsInAzure[azureGroupName]; !ok {
			fmt.Printf("Capability %s doesn't exist in Azure, creating.\n", rootId)
			resp, err := azureClient.CreateAdministrativeUnitGroup(aUnit.ID, rootId)
			if err != nil {
				log.Fatal(err)
			}

			azureGroup = &azure.Group{ID: resp.ID, DisplayName: resp.DisplayName}
			//continue
		} else {
			azureGroup = resp
			//fmt.Printf("Deleting group %s\n", azureGroup.DisplayName)
			//err = azureClient.DeleteAdministrativeUnitGroup(aUnit.ID, azureGroup.ID)
			//if err != nil {
			//	log.Fatal(err)
			//}
			//continue
		}

		// Add missing members in Azure AD group
		if azureGroup != nil {
			for _, capMember := range capability.Members {
				if !azureGroup.HasMember(capMember.Email) {
					fmt.Printf("Azure group %s missing member %s, adding.\n", azureGroup.DisplayName, capMember.Email)
					err = azureClient.AddGroupMember(azureGroup.ID, capMember.Email)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}

		// TODO: Check if member that exists in Azure AD group has been removed from the Capability, if that's the case, remove them.

	}
}

// SyncAzureToAws
//
// Currently in an unfinished state, can at the moment:
//   - Get groups
//   - Get members of a group
func SyncAzureToAws() {
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

		if !appAssignments.ContainsGroup(group.DisplayName) {
			fmt.Printf("Group %s has not been assigned to application yet, assigning.\n", group.DisplayName)
			_, err := azClient.AssignGroupToApplication(testData.Azure.ApplicationObjectId, group.ID, appRoleId)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

}

func SyncAwsToK8s() {
	testData := util.LoadTestData()

	resp := aws.GetSsoRoles(testData.AwsAccounts, testData.AssumableRoles.CapabilityAccountRoleName)
	for _, acc := range resp {
		fmt.Printf("AWS Account: %s\nSSO role name: %s\nSSO role arn: %s\n", acc.AccountAlias, acc.RoleName, acc.RoleArn)
	}

	amResp, err := k8s.LoadAwsAuthMapRoles()

	for _, mapping := range amResp.Mappings {
		if mapping.ManagedByThis() {
			fmt.Println(mapping)
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
