package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/ssu_exchange"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
)

const CapabilityEmailAliasName = "capabilityEmailAlias"

var SubAliases = []Alias{
	NewAlias("AWS Root", "aws-root", false),
	NewAlias("AWS Billing", "aws-billing", true),
	NewAlias("AWS Operations", "aws-operations", true),
	NewAlias("AWS Security", "aws-security", true),
}
var MainAlias = NewAlias("Root", "", false)

type Alias struct {
	DisplayName          string
	EmailAliasValue      string
	addCapabilityMembers bool
}

func NewAlias(displayName string, emailAliasValue string, addCapabilityMembers bool) Alias {
	return Alias{
		DisplayName:          displayName,
		EmailAliasValue:      emailAliasValue,
		addCapabilityMembers: addCapabilityMembers,
	}
}

var metricTotalEmailAliasCount = promauto.NewGauge(prometheus.GaugeOpts{
	Name:      "exchange_email_aliases_count",
	Help:      "Current email aliases",
	Namespace: "aad_aws_sync",
})

var metricTotalMissingEmailAliasCount = promauto.NewGauge(prometheus.GaugeOpts{
	Name:      "exchange_email_missing_aliases_count",
	Help:      "Missing email aliases",
	Namespace: "aad_aws_sync",
})

var metricEmailAliasCapabilitiesMismatch = promauto.NewGauge(prometheus.GaugeOpts{
	Name:      "exchange_email_alias_capabilities_mismatch",
	Help:      "email alias count - capability count",
	Namespace: "aad_aws_sync",
})

type missingAliasContainer struct {
	Alias            string
	GroupDisplayName string
	Capability       *capsvc.GetCapabilitiesResponseContextCapability
}

type state struct {
	EmailAliasesByDisplayName               map[string]ssu_exchange.GetAliasesResponse
	EmailAliasesByEmail                     map[string]ssu_exchange.GetAliasesResponse
	EmailAliasesWithoutCapabilities         map[string]ssu_exchange.GetAliasesResponse
	DistributionsGroupsInAzureByDisplayName map[string]*azure.Group
	MissingAliases                          []*missingAliasContainer
	CapabilitiesWithContextCount            int
}

func CapabilityEmailAliasHandler(ctx context.Context) error {
	util.Logger.Info("CapabilityEmailAliasHandler started")
	logger := util.Logger.With(zap.String("job", "CapabilityEmailAliasHandler"))
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	//
	// SETUP
	//
	capsvcClient := capsvc.NewCapSvcClient(capsvc.Config{
		Host:         conf.CapSvc.Host,
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.CapSvc.ClientId,
		ClientSecret: conf.CapSvc.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	//ssuClient := ssu_exchange.NewSsuExchangeClientPowershellWrapper(ssu_exchange.Config{
	//	TenantId:     conf.Azure.TenantId,
	//	ClientId:     conf.Azure.ClientId,
	//	ClientSecret: conf.Azure.ClientSecret,
	//	BaseUrl:      conf.Exchange.BaseUrl,
	//	ManagedBy:    conf.Exchange.ManagedBy,
	//})

	ssuClient := ssu_exchange.NewSsuExchangeClientO365UnofficialApi(ssu_exchange.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Exchange.ClientId,
		ClientSecret: conf.Exchange.ClientSecret,
		BaseUrl:      conf.Exchange.BaseUrl,
		ManagedBy:    conf.Exchange.ManagedBy,
		EmailSuffix:  conf.Exchange.EmailSuffix,
	})

	azClient := azure.NewAzureClient(azure.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
	})

	handler := &capabilityEmailAliasHandler{
		CapSvcClient:         capsvcClient,
		ExchangeOnlineClient: ssuClient,
		Cache:                &capabilityEmailAliasHandlerCache{},
	}

	capabilities, err := handler.CapSvcClient.GetCapabilities()
	if err != nil {
		return err
	}

	handler.Cache.Capabilities = capabilities

	// Get aliases from Exchange
	aliases, err := ssuClient.GetAliases(ctx)
	var aliasesByDisplayName map[string]ssu_exchange.GetAliasesResponse = make(map[string]ssu_exchange.GetAliasesResponse)
	var aliasesByEmail map[string]ssu_exchange.GetAliasesResponse = make(map[string]ssu_exchange.GetAliasesResponse)
	if err != nil {
		return err
	}

	if len(aliases) == 0 {
		return errors.New("0 aliases returned from Exchange Online. This is not expected behaviour")
	}

	metricTotalEmailAliasCount.Set(float64(len(aliases)))
	// Put aliases in map
	for _, alias := range aliases {
		aliasesByDisplayName[alias.Identity] = alias
		aliasesByEmail[alias.WindowsEmailAddress] = alias
	}

	handlerState := &state{
		EmailAliasesByDisplayName:               aliasesByDisplayName,
		EmailAliasesByEmail:                     aliasesByEmail,
		EmailAliasesWithoutCapabilities:         map[string]ssu_exchange.GetAliasesResponse{},
		DistributionsGroupsInAzureByDisplayName: map[string]*azure.Group{},
		MissingAliases:                          []*missingAliasContainer{},
	}
	handler.State = handlerState

	// Get corresponding groups in Azure for aliases
	azureGroupsResp, err := azClient.GetGroups("CI_SSU_Ex")
	if err != nil {
		return err
	}

	for _, grp := range azureGroupsResp.Value {
		group := &azure.Group{
			DisplayName: grp.DisplayName,
			Members:     []*azure.Member{},
			ID:          grp.ID,
		}
		groupMembers, err := azClient.GetGroupMembers(grp.ID)
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

		handlerState.DistributionsGroupsInAzureByDisplayName[group.DisplayName] = group
	}

	//
	// SETUP END
	//

	// check for main alias  create if it doesn't exist, reconcile group members
	for _, capa := range handler.Cache.Capabilities {
		displayName := ssu_exchange.GenerateExchangeDistributionGroupDisplayName(fmt.Sprintf("%s %s", capa.RootID, MainAlias.DisplayName))
		exists := handler.AliasDisplayNameExists(displayName)
		if exists {
			// Reconcile group members
			capaWithCcMembers := capa
			capaWithCcMembers.Members = append(capaWithCcMembers.Members, capsvc.GetCapabilitiesResponseContextCapabilityMember{Email: conf.Exchange.CcEmail})
			dstGroup, exists := handlerState.DistributionsGroupsInAzureByDisplayName[displayName]
			if !exists {
				return errors.New("alias exists in Exchange Online, but equivalent group in Azure doesn't, something is off")
			}

			for _, capabilityMember := range capaWithCcMembers.Members {
				if !dstGroup.HasMember(capabilityMember.Email) {
					logger.Info(fmt.Sprintf("%s missing from exchange alias %s, adding", capabilityMember.Email, capa.RootID))
					err = ssuClient.AddDistributionGroupMember(ctx, fmt.Sprintf("%s %s", capa.RootID, MainAlias.DisplayName), capabilityMember.Email)
					if err != nil {
						return err
					}
				}
			}

			for _, azGrpMember := range dstGroup.Members {
				if !capaWithCcMembers.HasMember(azGrpMember.UserPrincipalName) {
					logger.Info(fmt.Sprintf("exchange alias %s contains stale member %s, removing", capa.RootID, azGrpMember.UserPrincipalName))
					err = ssuClient.RemoveDistributionGroupMember(ctx, fmt.Sprintf("%s %s", capa.RootID, MainAlias.DisplayName), azGrpMember.UserPrincipalName)
					if err != nil {
						return err
					}
				}
			}

		} else {
			// Create alias, add members
			members := memberStringBuilder(capa.Members)
			err = ssuClient.CreateAlias(ctx, capa.RootID, fmt.Sprintf("%s %s", capa.RootID, MainAlias.DisplayName), members)
			if err != nil {
				return err
			}
			logger.Info(fmt.Sprintf("created email alias %s.ssu for %s", capa.RootID, capa.RootID))
		}
	}

	// Check for sub aliases, create if they don't exist
	for _, capa := range handler.Cache.Capabilities {
		for _, subAlias := range SubAliases {
			displayName := ssu_exchange.GenerateExchangeDistributionGroupDisplayName(fmt.Sprintf("%s %s", capa.RootID, subAlias.DisplayName))
			exists := handler.AliasDisplayNameExists(displayName)

			if !exists {
				members := []string{}
				if subAlias.addCapabilityMembers {
					members = append(members, fmt.Sprintf("%s%s", capa.RootID, conf.Exchange.EmailSuffix))
				} else {
					members = append(members, conf.Exchange.CcEmail)
				}
				err = ssuClient.CreateAlias(ctx, fmt.Sprintf("%s.%s", subAlias.EmailAliasValue, capa.RootID), fmt.Sprintf("%s %s", capa.RootID, subAlias.DisplayName), members)
				if err != nil {
					return err
				}
				logger.Info(fmt.Sprintf("created email alias %s for %s", fmt.Sprintf("%s.%s", subAlias.EmailAliasValue, capa.RootID), capa.RootID))

			}
		}
	}

	// Check for legacy aliases, create if they don't exist

	return nil
}

type capabilityEmailAliasHandler struct {
	CapSvcClient         *capsvc.Client
	ExchangeOnlineClient ssu_exchange.IClient
	Cache                *capabilityEmailAliasHandlerCache
	State                *state
}

type capabilityEmailAliasHandlerCache struct {
	Capabilities []*capsvc.GetCapabilitiesResponseContextCapability
}

func (c *capabilityEmailAliasHandler) Reconcile() {

}

func (c *capabilityEmailAliasHandler) CreateMainAlias() {

}

func (c *capabilityEmailAliasHandler) CreateSubAliases() {

}

func (c *capabilityEmailAliasHandler) EmailAliasExists(value string) bool {
	_, ok := c.State.EmailAliasesByEmail[value]
	return ok
}

func (c *capabilityEmailAliasHandler) AliasDisplayNameExists(value string) bool {
	_, ok := c.State.EmailAliasesByDisplayName[value]
	return ok
}

func memberStringBuilder(data []capsvc.GetCapabilitiesResponseContextCapabilityMember) []string {
	members := []string{}
	for _, member := range data {
		members = append(members, member.Email)
	}

	return members
}
