package azure

import "time"

type GroupsListResponse struct {
	OdataContext  string `json:"@odata.context"`
	OdataNextLink string `json:"@odata.nextLink"`
	Value         []struct {
		ID                            string        `json:"id"`
		DeletedDateTime               interface{}   `json:"deletedDateTime"`
		Classification                interface{}   `json:"classification"`
		CreatedDateTime               time.Time     `json:"createdDateTime"`
		CreationOptions               []interface{} `json:"creationOptions"`
		Description                   string        `json:"description"`
		DisplayName                   string        `json:"displayName"`
		ExpirationDateTime            interface{}   `json:"expirationDateTime"`
		GroupTypes                    []interface{} `json:"groupTypes"`
		IsAssignableToRole            interface{}   `json:"isAssignableToRole"`
		Mail                          interface{}   `json:"mail"`
		MailEnabled                   bool          `json:"mailEnabled"`
		MailNickname                  string        `json:"mailNickname"`
		MembershipRule                interface{}   `json:"membershipRule"`
		MembershipRuleProcessingState interface{}   `json:"membershipRuleProcessingState"`
		OnPremisesDomainName          string        `json:"onPremisesDomainName"`
		OnPremisesLastSyncDateTime    time.Time     `json:"onPremisesLastSyncDateTime"`
		OnPremisesNetBiosName         string        `json:"onPremisesNetBiosName"`
		OnPremisesSamAccountName      string        `json:"onPremisesSamAccountName"`
		OnPremisesSecurityIdentifier  string        `json:"onPremisesSecurityIdentifier"`
		OnPremisesSyncEnabled         bool          `json:"onPremisesSyncEnabled"`
		PreferredDataLocation         interface{}   `json:"preferredDataLocation"`
		PreferredLanguage             interface{}   `json:"preferredLanguage"`
		ProxyAddresses                []interface{} `json:"proxyAddresses"`
		RenewedDateTime               time.Time     `json:"renewedDateTime"`
		ResourceBehaviorOptions       []interface{} `json:"resourceBehaviorOptions"`
		ResourceProvisioningOptions   []interface{} `json:"resourceProvisioningOptions"`
		SecurityEnabled               bool          `json:"securityEnabled"`
		SecurityIdentifier            string        `json:"securityIdentifier"`
		Theme                         interface{}   `json:"theme"`
		Visibility                    interface{}   `json:"visibility"`
		OnPremisesProvisioningErrors  []interface{} `json:"onPremisesProvisioningErrors"`
	} `json:"value"`
}
