package handler

import (
	"context"
	"errors"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
)

const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"
const CapabilityServiceToAzureAdName = "capSvcToAad"

func Capsvc2AadHandler(ctx context.Context) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	groupsInAzure := make(map[string]*azure.Group)
	capabilitiesByRootId := make(map[string]*capsvc.GetCapabilitiesResponseContextCapability)
	client := capsvc.NewCapSvcClient(capsvc.Config{
		Host:         conf.CapSvc.Host,
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	capabilities, err := client.GetCapabilities()
	if err != nil {
		return err
	}

	azureClient := azure.NewAzureClient(azure.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
	})

	aUnits, err := azureClient.GetAdministrativeUnits()
	if err != nil {
		return err
	}

	aUnit := aUnits.GetUnit("Team - Cloud Engineering - Self service")
	if aUnit == nil {
		return errors.New("unable to find administrative unit")
	}

	aUnitMembers, err := azureClient.GetAdministrativeUnitMembers(aUnit.ID)
	if err != nil {
		return err
	}

	for _, capability := range capabilities.Items {
		_, err := capability.GetContext()
		if err == nil {
			capabilitiesByRootId[capability.RootID] = capability
		}
	}

	for _, member := range aUnitMembers.Value {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", CapabilityServiceToAzureAdName))
			return nil
		default:
			group := &azure.Group{
				DisplayName: member.DisplayName,
				ID:          member.ID,
				Members:     []*azure.Member{},
			}
			groupMembers, err := azureClient.GetGroupMembers(member.ID)
			if err != nil {
				return err
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
	}

	for rootId, capability := range capabilitiesByRootId {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", CapabilityServiceToAzureAdName))
			return nil
		default:
		}
		azureGroupName := fmt.Sprintf("%s %s", CAPABILITY_GROUP_PREFIX, rootId)
		var azureGroup *azure.Group

		// Check if Capability has a group in Azure AD, if it doesn't create it
		if resp, ok := groupsInAzure[azureGroupName]; !ok {
			fmt.Printf("Capability %s doesn't exist in Azure, creating.\n", rootId)
			createGroupRequest := azure.CreateAdministrativeUnitGroupRequest{
				OdataType:       "#Microsoft.Graph.Group",
				Description:     "[Automated] - aad-aws-sync",
				DisplayName:     azure.GenerateAzureGroupDisplayName(rootId),
				MailNickname:    azure.GenerateAzureGroupMailPrefix(rootId),
				GroupTypes:      []interface{}{},
				MailEnabled:     false,
				SecurityEnabled: true,

				ParentAdministrativeUnitId: aUnit.ID,
			}
			resp, err := azureClient.CreateAdministrativeUnitGroup(ctx, createGroupRequest)
			if err != nil {
				return err
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
				select {
				case <-ctx.Done():
					util.Logger.Info("Job cancelled", zap.String("jobName", CapabilityServiceToAzureAdName))
					return nil
				default:
				}
				if !azureGroup.HasMember(capMember.Email) {
					util.Logger.Info(fmt.Sprintf("Azure group %s missing member %s, adding.\n", azureGroup.DisplayName, capMember.Email), zap.String("jobName", CapabilityServiceToAzureAdName))
					err = azureClient.AddGroupMember(azureGroup.ID, capMember.Email)
					if err != nil {
						return err
					}
				}
			}
		}

		// TODO: Check if member that exists in Azure AD group has been removed from the Capability, if that's the case, remove them.

	}
	return nil
}
