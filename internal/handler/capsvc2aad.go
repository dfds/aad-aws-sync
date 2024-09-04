package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/joomcode/errorx"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"sync"
)

const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"
const CapabilityServiceToAzureAdName = "capSvc2Aad"

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
		ClientId:     conf.CapSvc.ClientId,
		ClientSecret: conf.CapSvc.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	capabilities, err := client.GetCapabilities()
	if err != nil {
		return err
	}

	azureClient := azure.NewAzureClient(azure.Config{
		TenantId:             conf.Azure.TenantId,
		ClientId:             conf.Azure.ClientId,
		ClientSecret:         conf.Azure.ClientSecret,
		InternalDomainSuffix: conf.Azure.InternalDomainSuffix,
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

	for _, capability := range capabilities {
		capabilitiesByRootId[capability.RootID] = capability
	}

	{
		ctx := context.Background()
		var waitGroup sync.WaitGroup
		sem := semaphore.NewWeighted(50)
		var lock *sync.Mutex = &sync.Mutex{}
		for _, member := range aUnitMembers.Value {
			select {
			case <-ctx.Done():
				util.Logger.Info("Job cancelled", zap.String("jobName", CapabilityServiceToAzureAdName))
				return nil
			default:
				waitGroup.Add(1)
				member := member
				go func() error {
					sem.Acquire(ctx, 1)
					defer sem.Release(1)
					defer waitGroup.Done()

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

					lock.Lock()
					groupsInAzure[group.DisplayName] = group
					lock.Unlock()
					return nil
				}()
			}
		}

		waitGroup.Wait()
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
			util.Logger.Info(fmt.Sprintf("Capability %s doesn't exist in Azure, creating.\n", rootId), zap.String("jobName", CapabilityServiceToAzureAdName))
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
		} else {
			azureGroup = resp
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

				upn := capMember.Email
				// treat user as external, look up UPN manually
				if azureClient.IsUserExternal(upn) {
					resp, err := azureClient.GetUserViaEmail(capMember.Email)
					if err != nil {
						return err
					}
					upn = resp.UserPrincipalName
				}

				if !azureGroup.HasMember(upn) {
					util.Logger.Debug(fmt.Sprintf("Azure group %s missing member %s, adding.\n", azureGroup.DisplayName, upn), zap.String("jobName", CapabilityServiceToAzureAdName))

					err = azureClient.AddGroupMember(azureGroup.ID, upn)
					if err != nil {
						if errorx.IsOfType(err, azure.AdUserNotFound) {
							util.Logger.Debug(err.Error(), zap.String("jobName", CapabilityServiceToAzureAdName))
							continue
						}
						if errorx.IsOfType(err, azure.HttpError403) {
							util.Logger.Debug(err.Error(), zap.String("jobName", CapabilityServiceToAzureAdName))
							continue
						}

						return err
					}
				}
			}

			//Delete members no longer in capability from Azure AD Group
			for _, member := range azureGroup.Members {
				select {
				case <-ctx.Done():
					util.Logger.Info("Job cancelled", zap.String("jobName", CapabilityServiceToAzureAdName))
					return nil
				default:
				}

				upn := member.UserPrincipalName
				// treat user as external, look up UPN manually
				if azureClient.IsUserExternal(upn) {
					resp, err := azureClient.GetUserViaUPN(member.UserPrincipalName)
					if err != nil {
						return err
					}
					upn = resp.Mail
				}

				if !capability.HasMember(upn) {
					util.Logger.Debug(fmt.Sprintf("Azure group %s contains stale member %s, removing.\n", azureGroup.DisplayName, upn), zap.String("jobName", CapabilityServiceToAzureAdName))
					err = azureClient.DeleteGroupMember(azureGroup.ID, member.ID)
					if err != nil {
						return err
					}
				}
			}

		}
	}
	return nil
}
