package handler

import (
	"context"
	"fmt"
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
	client := ssu_exchange.NewSsuExchangeClient(ssu_exchange.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
		BaseUrl:      "http://localhost:7071/api",
	})

	aliases, err := client.GetAliases(ctx)
	if err != nil {
		return err
	}

	for _, alias := range aliases {
		fmt.Printf("%s (%s)\n", alias.Identity, alias.PrimarySMTPAddress)
	}

	return nil
}
