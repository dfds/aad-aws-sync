package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Aws struct {
		IdentityStoreArn               string `json:"identityStoreArn"`
		SsoInstanceArn                 string `json:"ssoInstanceArn"`
		CapabilityPermissionSetArn     string `json:"capabilityPermissionSetArn"`
		CapabilityLogsPermissionSetArn string `json:"capabilityLogsPermissionSetArn"`
		CapabilityLogsAwsAccountAlias  string `json:"capabilityLogsAwsAccountAlias"`
		SharedEcrPullPermissionSetArn  string `json:"sharedEcrPullPermissionSetArn"`
		SharedEcrPullAwsAccountAlias   string `json:"sharedEcrPullAwsAccountAlias"`
		AccountNamePrefix              string `json:"accountNamePrefix"`
		SsoRegion                      string `json:"ssoRegion"`
		AssumableRoles                 struct {
			SsoManagementArn          string `json:"ssoManagementArn"`
			CapabilityAccountRoleName string `json:"capabilityAccountRoleName"`
		} `json:"assumableRoles"`
		Scim struct {
			Endpoint string `json:"endpoint"`
			Token    string `json:"token"`
		}
		OrganizationsParentId     string `json:"organizationsParentId"`
		RootOrganizationsParentId string `json:"rootOrganizationsParentId"`
	} `json:"aws"`
	Azure struct {
		TenantId            string `json:"tenantId"`
		ClientId            string `json:"clientId"`
		ClientSecret        string `json:"clientSecret"`
		ApplicationId       string `json:"applicationId"`
		ApplicationObjectId string `json:"applicationObjectId"`
	} `json:"azure"`
	CapSvc struct { // Capability-Service
		Host         string `json:"host"`
		TokenScope   string `json:"tokenScope"`
		ClientId     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	} `json:"capSvc"`
	Exchange struct {
		ManagedBy   string `json:"managedBy"`
		BaseUrl     string `json:"baseUrl"`
		EmailSuffix string `json:"emailSuffix"`
		CcEmail     string `json:"ccEmail"`
	}
	Handler struct {
		AssignGroups2AzureEnterpriseApps struct {
			DataFilePath string `json:"dataFilePath"`
		} `json:"assignGroups2AzureEnterpriseApps"`
	} `json:"handler"`
	Log struct {
		Level string `json:"level"`
		Debug bool   `json:"debug"`
	}
	EventHandling struct {
		Enabled bool `json:"enable"`
	}
	Scheduler struct {
		Frequency                  string `json:"scheduleFrequency" default:"30m"`
		EnableCapsvc2Azure         bool   `json:"enableCapsvc2Azure"`
		EnableAzure2Aws            bool   `json:"enableAzure2Aws"`
		EnableAwsMapping           bool   `json:"enableAwsMapping"`
		EnableAws2K8s              bool   `json:"enableAws2K8S"`
		EnableCapabilityEmailAlias bool   `json:"enableCapabilityEmailAlias"`
	}
}

const APP_CONF_PREFIX = "AAS"

func LoadConfig() (Config, error) {
	var conf Config
	err := envconfig.Process(APP_CONF_PREFIX, &conf)

	return conf, err
}
