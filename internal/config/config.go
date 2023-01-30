package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Aws struct {
		IdentityStoreArn               string `json:"identityStoreArn"`
		SsoInstanceArn                 string `json:"ssoInstanceArn"`
		CapabilityPermissionSetArn     string `json:"capabilityPermissionSetArn"`
		CapabilityLogsPermissionSetArn string `json:"capabilityLogsPermissionSetArn"`
		CapabilityLogsAwsAccountAlias  string `json:"capabilityLogsAwsAccountAlias"`
		AccountNamePrefix              string `json:"accountNamePrefix"`
		SsoRegion                      string `json:"ssoRegion"`
		AssumableRoles                 struct {
			SsoManagementArn          string `json:"ssoManagementArn"`
			CapabilityAccountRoleName string `json:"capabilityAccountRoleName"`
		} `json:"assumableRoles"`
	} `json:"aws"`
	Azure struct {
		TenantId            string `json:"tenantId"`
		ClientId            string `json:"clientId"`
		ClientSecret        string `json:"clientSecret"`
		ApplicationId       string `json:"applicationId"`
		ApplicationObjectId string `json:"applicationObjectId"`
	} `json:"azure"`
	CapSvc struct { // Capability-Service
		Host       string `json:"host"`
		TokenScope string `json:"tokenScope"`
	} `json:"capSvc"`
	Scheduler struct {
		Frequency string `json:"scheduleFrequency" default:"30m"`
	}
}

const APP_CONF_PREFIX = "AAS"

func LoadConfig() (Config, error) {
	var conf Config
	err := envconfig.Process(APP_CONF_PREFIX, &conf)

	return conf, err
}
