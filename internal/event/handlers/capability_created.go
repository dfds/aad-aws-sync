package handlers

import (
	"context"
	"errors"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/event/model"
	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
)

type capabilityCreated struct {
	CapabilityID   string `json:"capabilityId"`
	CapabilityName string `json:"capabilityName"`
}

func CapabilityCreatedHandler(ctx context.Context, event model.HandlerContext) error {
	msgLog := util.Logger.With(zap.String("event_handler", "CapabilityCreatedHandler"), zap.String("event", event.Event.Name))
	msgLog.Info("New Capability discovered. Creating entries in AAD")
	msg, err := GetEventWithPayloadFromMsg[capabilityCreated](event.Msg)
	if err != nil {
		return err
	}
	msgLog = msgLog.With(zap.String("capabilityId", msg.Payload.CapabilityID))

	var capability *capsvc.GetCapabilitiesResponseContextCapability

	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	groupsInAzure := make(map[string]*azure.Group)
	client := capsvc.NewCapSvcClient(capsvc.Config{
		Host:         conf.CapSvc.Host,
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.CapSvc.ClientId,
		ClientSecret: conf.CapSvc.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	capabilities, err := client.GetCapabilities()
	if err != nil {
		return err
	}

	for _, capa := range capabilities {
		if capa.ID == msg.Payload.CapabilityID {
			capability = capa
			break
		}
	}

	if capability == nil {
		return errors.New("capability from event not found in Capability-Service")
	}
	msgLog = msgLog.With(zap.String("capabilityRootId", capability.RootID))

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

	for _, member := range aUnitMembers.Value {
		select {
		case <-ctx.Done():
			msgLog.Info("Job cancelled")
			return errors.New("event handling cancelled via context")
		default:
			group := &azure.Group{
				DisplayName: member.DisplayName,
				ID:          member.ID,
				Members:     []*azure.Member{},
			}
			groupsInAzure[group.DisplayName] = group
		}
	}

	azureGroupName := fmt.Sprintf("%s %s", handler.CAPABILITY_GROUP_PREFIX, capability.RootID)
	var azureGroup *azure.Group
	// Check if Capability has a group in Azure AD, if it doesn't create it
	if resp, ok := groupsInAzure[azureGroupName]; !ok {
		msgLog.Info(fmt.Sprintf("Capability %s doesn't exist in Azure, creating.", capability.RootID))
		createGroupRequest := azure.CreateAdministrativeUnitGroupRequest{
			OdataType:       "#Microsoft.Graph.Group",
			Description:     "[Automated] - aad-aws-sync",
			DisplayName:     azure.GenerateAzureGroupDisplayName(capability.RootID),
			MailNickname:    azure.GenerateAzureGroupMailPrefix(capability.RootID),
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
	} else {
		azureGroup = resp
	}

	select {
	case <-ctx.Done():
		msgLog.Info("Job cancelled")
		return errors.New("event handling cancelled via context")
	default:
	}

	appRoles, err := azureClient.GetApplicationRoles(conf.Azure.ApplicationId)
	if err != nil {
		return err
	}

	appRoleId, err := appRoles.GetRoleId("User")
	if err != nil {
		return err
	}

	appAssignments, err := azureClient.GetAssignmentsForApplication(conf.Azure.ApplicationObjectId)
	if err != nil {
		return err
	}

	if !appAssignments.ContainsGroup(azureGroup.DisplayName) {
		msgLog.Info(fmt.Sprintf("Group %s has not been assigned to application yet, assigning", azureGroup.DisplayName))
		_, err := azureClient.AssignGroupToApplication(conf.Azure.ApplicationObjectId, azureGroup.ID, appRoleId)
		if err != nil {
			return err
		}
	}

	return nil
}
