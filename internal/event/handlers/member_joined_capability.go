package handlers

import (
	"context"
	"errors"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/aws"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/event/model"
	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
)

type memberJoinedCapability struct {
	CapabilityID string `json:"capabilityId"`
	MembershipID string `json:"membershipId"`
	UserID       string `json:"userId"`
}

func MemberJoinedCapabilityHandler(ctx context.Context, event model.HandlerContext) error {
	msgLog := util.Logger.With(zap.String("event_handler", "MemberJoinedCapabilityHandler"), zap.String("event", event.Event.Type))
	msg, err := GetEventWithPayloadFromMsg[memberJoinedCapability](event.Msg)
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
	capSvcClient := capsvc.NewCapSvcClient(capsvc.Config{
		Host:         conf.CapSvc.Host,
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.CapSvc.ClientId,
		ClientSecret: conf.CapSvc.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	azureClient := azure.NewAzureClient(azure.Config{
		TenantId:             conf.Azure.TenantId,
		ClientId:             conf.Azure.ClientId,
		ClientSecret:         conf.Azure.ClientSecret,
		InternalDomainSuffix: conf.Azure.InternalDomainSuffix,
	})

	scimClient := aws.CreateScimClient(conf.Aws.Scim.Endpoint, conf.Aws.Scim.Token)

	capabilities, err := capSvcClient.GetCapabilities()
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
	msgLog.Info(fmt.Sprintf("%s joined Capability %s. Updating AAD group", msg.Payload.UserID, capability.RootID))

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

	err = azureClient.AddGroupMember(azureGroup.ID, msg.Payload.UserID)
	if err != nil {
		return err
	}

	aadUser, err := azureClient.GetUserViaUPN(msg.Payload.UserID)
	if err != nil {
		return err
	}

	scimUser, err := scimClient.GetUserViaExternalId(aadUser.ID)
	if err != nil {
		msgLog.Info("User is not yet provisioned to AWS. Skipping direct provisioning, letting Azure handle the initial provisioning of user and memberships")
		return nil
	}

	scimGroup, err := scimClient.GetGroupViaDisplayName(azureGroupName)
	if err != nil {
		msgLog.Info("Group is not yet provisioned to AWS. Skipping direct provisioning, letting Azure handle the initial provisioning of group")
		return nil
	}

	err = scimClient.PatchAddMembersToGroup(scimGroup.ID, scimUser.ID)
	if err != nil {
		return err
	}

	return nil
}
