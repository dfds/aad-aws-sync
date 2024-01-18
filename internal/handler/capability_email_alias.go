package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/ssu_exchange"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

const CapabilityEmailAliasName = "capabilityEmailAlias"

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

	if len(aliases) == 0 {
		return errors.New("0 aliases returned from Exchange Online. This is not expected behaviour")
	}

	metricTotalEmailAliasCount.Set(float64(len(aliases)))
	for _, alias := range aliases {
		emailAliasesMapped[alias.Identity] = alias
	}

	var capabilitiesWithMissingEmailAlias []*capsvc.GetCapabilitiesResponseContextCapability
	var capabilitiesWithContextCount = 0
	var emailAliasesWithoutCapabilities map[string]ssu_exchange.GetAliasesResponse = make(map[string]ssu_exchange.GetAliasesResponse)
	for k, v := range emailAliasesMapped {
		emailAliasesWithoutCapabilities[k] = v
	}

	for _, capa := range capabilities {
		if _, ok := emailAliasesWithoutCapabilities[ssu_exchange.GenerateExchangeDistributionGroupDisplayName(capa.RootID)]; ok {
			delete(emailAliasesWithoutCapabilities, ssu_exchange.GenerateExchangeDistributionGroupDisplayName(capa.RootID))
		}

		_, err := capa.GetContext()
		if err != nil {
			continue
		}
		capabilitiesWithContextCount = capabilitiesWithContextCount + 1

		if _, ok := emailAliasesMapped[ssu_exchange.GenerateExchangeDistributionGroupDisplayName(capa.RootID)]; !ok {
			capabilitiesWithMissingEmailAlias = append(capabilitiesWithMissingEmailAlias, capa)
			util.Logger.Info(fmt.Sprintf("%s missing email alias", capa.RootID))
		}
	}
	metricTotalMissingEmailAliasCount.Set(float64(len(capabilitiesWithMissingEmailAlias)))
	metricEmailAliasCapabilitiesMismatch.Set(float64(len(aliases) - capabilitiesWithContextCount))

	for _, capa := range capabilitiesWithMissingEmailAlias {
		err = ssuClient.CreateAlias(ctx, capa.RootID)
		if err != nil {
			return err
		}
		util.Logger.Info(fmt.Sprintf("Created email alias for %s", capa.RootID))

	}

	if len(emailAliasesWithoutCapabilities) > 0 {
		util.Logger.Debug("Email aliases without a corresponding Capability")
		for k := range emailAliasesWithoutCapabilities {
			util.Logger.Debug(k)
		}
	}

	return nil
}
