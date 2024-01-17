package handler

import (
	"context"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/ssu_exchange"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

const CapabilityEmailAliasName = "capabilityEmailAlias"

func CapabilityEmailAliasHandler(ctx context.Context) error {
	util.Logger.Info("CapabilityEmailAliasHandler started")
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	capsvcClient := capsvc.NewCapSvcClient(capsvc.Config{
		Host:         conf.CapSvc.Host,
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.CapSvc.ClientId,
		ClientSecret: conf.CapSvc.ClientSecret,
		Scope:        conf.CapSvc.TokenScope,
	})

	capabilities, err := capsvcClient.GetCapabilities()
	if err != nil {
		return err
	}

	ssuClient := ssu_exchange.NewSsuExchangeClient(ssu_exchange.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
		BaseUrl:      conf.Exchange.BaseUrl,
		ManagedBy:    conf.Exchange.ManagedBy,
	})

	aliases, err := ssuClient.GetAliases(ctx)
	var emailAliasesMapped map[string]ssu_exchange.GetAliasesResponse = make(map[string]ssu_exchange.GetAliasesResponse)
	if err != nil {
		return err
	}

	for _, alias := range aliases {
		emailAliasesMapped[alias.Identity] = alias
	}

	var capabilitiesWithMissingEmailAlias []*capsvc.GetCapabilitiesResponseContextCapability

	for _, capa := range capabilities {
		_, err := capa.GetContext()
		if err != nil {
			continue
		}

		if _, ok := emailAliasesMapped[ssu_exchange.GenerateExchangeDistributionGroupDisplayName(capa.RootID)]; !ok {
			capabilitiesWithMissingEmailAlias = append(capabilitiesWithMissingEmailAlias, capa)
			util.Logger.Info(fmt.Sprintf("%s missing email alias", capa.RootID))
		}
	}

	for _, capa := range capabilitiesWithMissingEmailAlias {
		err = ssuClient.CreateAlias(ctx, capa.RootID)
		if err != nil {
			return err
		}
		util.Logger.Info(fmt.Sprintf("Created email alias for %s", capa.RootID))

	}

	return nil
}
