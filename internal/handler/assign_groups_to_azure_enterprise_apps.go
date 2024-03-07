package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joomcode/errorx"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"os"
)

const AssignGroupsToAzureEnterpriseAppsName = "assignGroups2AzureEnterpriseApps"

type EnterpriseAppData struct {
	AppId    string `json:"appId"`
	ObjectId string `json:"objectId"`
}

func loadEnterpriseAppMappings(path string) ([]EnterpriseAppData, error) {
	if path == "" {
		return nil, DataPathNotConfigured.New("Data path not configured, unable to load enterprise app mappings")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var payload []EnterpriseAppData

	err = json.Unmarshal(data, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func AssignGroupsToAzureEnterpriseAppsHandler(ctx context.Context) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	apps, err := loadEnterpriseAppMappings(conf.Handler.AssignGroups2AzureEnterpriseApps.DataFilePath)
	if err != nil {
		return err
	}

	azClient := azure.NewAzureClient(azure.Config{
		TenantId:     conf.Azure.TenantId,
		ClientId:     conf.Azure.ClientId,
		ClientSecret: conf.Azure.ClientSecret,
	})

	groups, err := azClient.GetGroups(azure.AZURE_CAPABILITY_GROUP_PREFIX)
	if err != nil {
		return err
	}

	for _, app := range apps {
		appRoles, err := azClient.GetApplicationRoles(app.AppId)
		if err != nil {
			return err
		}

		appRoleId, err := appRoles.GetRoleId("User")
		if err != nil {
			return err
		}

		appAssignments, err := azClient.GetAssignmentsForApplication(app.ObjectId)
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

			util.Logger.Debug(group.DisplayName, zap.String("jobName", AzureAdToAwsName))

			// If group is not already assigned to enterprise application, assign them.
			if !appAssignments.ContainsGroup(group.DisplayName) {
				util.Logger.Info(fmt.Sprintf("Group %s has not been assigned to application yet, assigning", group.DisplayName), zap.String("jobName", AzureAdToAwsName))
				_, err := azClient.AssignGroupToApplication(app.ObjectId, group.ID, appRoleId)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

var (
	AssignGroupsToAzureEnterpriseAppsError = errorx.NewNamespace("assignGroups2AzureEnterpriseApps")
	DataPathNotConfigured                  = AssignGroupsToAzureEnterpriseAppsError.NewType("data_path_not_configured")
)
