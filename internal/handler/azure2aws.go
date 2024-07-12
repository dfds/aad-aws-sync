package handler

import (
	"context"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
)

const AzureAdToAwsName = "aad2Aws"

func Azure2AwsHandler(ctx context.Context) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	azClient := azure.NewAzureClient(azure.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
	})

	capabilitiesByRootId := make(map[string]*capsvc.GetCapabilitiesResponseContextCapability)
	ssuClient := capsvc.NewCapSvcClient(capsvc.Config{
		Host:         conf.CapSvc.Host,
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.CapSvc.ClientId,
		ClientSecret: conf.CapSvc.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	capabilities, err := ssuClient.GetCapabilities()
	if err != nil {
		return err
	}

	for _, capability := range capabilities {
		_, err := capability.GetContext()
		if err == nil {
			capabilitiesByRootId[fmt.Sprintf("%s %s", azure.AZURE_CAPABILITY_GROUP_PREFIX, capability.RootID)] = capability
		}
	}

	appRoles, err := azClient.GetApplicationRoles(conf.Azure.ApplicationId)
	if err != nil {
		return err
	}

	appRoleId, err := appRoles.GetRoleId("User")
	if err != nil {
		return err
	}

	appAssignments, err := azClient.GetAssignmentsForApplication(conf.Azure.ApplicationObjectId)
	if err != nil {
		return err
	}

	groups, err := azClient.GetGroups(azure.AZURE_CAPABILITY_GROUP_PREFIX)
	if err != nil {
		return err
	}

	for _, group := range groups.Value {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", AzureAdToAwsName))
			return nil
		default:
		}

		// Skip if group doesn't have a context
		if _, ok := capabilitiesByRootId[group.DisplayName]; !ok {
			continue
		}

		util.Logger.Debug(group.DisplayName, zap.String("jobName", AzureAdToAwsName))

		// If group is not already assigned to enterprise application, assign them.
		if !appAssignments.ContainsGroup(group.DisplayName) {
			util.Logger.Info(fmt.Sprintf("Group %s has not been assigned to application yet, assigning", group.DisplayName), zap.String("jobName", AzureAdToAwsName))
			_, err := azClient.AssignGroupToApplication(conf.Azure.ApplicationObjectId, group.ID, appRoleId)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
