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

type memberLeftCapability struct {
	CapabilityId string `json:"capabilityId"`
	MemberEmail  string `json:"memberEmail"`
}

func MemberLeftCapabilityHandler(ctx context.Context, event model.HandlerContext) error {
	msgLog := util.Logger.With(zap.String("event_handler", "MemberLeftCapabilityHandler"), zap.String("event", event.Event.Name))
	msg, err := GetEventWithPayloadFromMsg[memberLeftCapability](event.Msg)
	if err != nil {
		return err
	}
	msgLog = msgLog.With(zap.String("capabilityId", msg.Payload.CapabilityId))

	var capability *capsvc.GetCapabilitiesResponseContextCapability

	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	groupsInAzure := make(map[string]*azure.Group)
	capSvcClient := capsvc.NewCapSvcClient(capsvc.Config{
		Host:         conf.CapSvc.Host,
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	azureClient := azure.NewAzureClient(azure.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
	})

	capabilities, err := capSvcClient.GetCapabilities()
	if err != nil {
		return err
	}

	for _, capa := range capabilities.Items {
		if capa.ID == msg.Payload.CapabilityId {
			capability = capa
			break
		}
	}

	if capability == nil {
		return errors.New("capability from event not found in Capability-Service")
	}
	msgLog = msgLog.With(zap.String("capabilityRootId", capability.RootID))
	msgLog.Info(fmt.Sprintf("%s left Capability %s. Updating AAD group", msg.Payload.MemberEmail, capability.RootID))

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
	if resp, ok := groupsInAzure[azureGroupName]; !ok {
		return errors.New(fmt.Sprintf("Capability %s doesn't exist in Azure. Unable to add new member", capability.RootID))
	} else {
		azureGroup = resp
	}

	aadUser, err := azureClient.GetUserViaUPN(msg.Payload.MemberEmail)
	if err != nil {
		return err
	}
	err = azureClient.DeleteGroupMember(azureGroup.ID, aadUser.ID)
	if err != nil {
		return err
	}

	return nil
}
