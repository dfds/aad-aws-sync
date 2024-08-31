package ssu_exchange

import "time"

type GetAliasesResponse struct {
	GroupType                                              string        `json:"GroupType"`
	SamAccountName                                         string        `json:"SamAccountName"`
	BypassNestedModerationEnabled                          bool          `json:"BypassNestedModerationEnabled"`
	IsDirSynced                                            bool          `json:"IsDirSynced"`
	ManagedBy                                              []string      `json:"ManagedBy"`
	MemberJoinRestriction                                  string        `json:"MemberJoinRestriction"`
	MemberDepartRestriction                                string        `json:"MemberDepartRestriction"`
	MigrationToUnifiedGroupInProgress                      bool          `json:"MigrationToUnifiedGroupInProgress"`
	HiddenGroupMembershipEnabled                           bool          `json:"HiddenGroupMembershipEnabled"`
	ExpansionServer                                        string        `json:"ExpansionServer"`
	AcceptMessagesOnlyFromWithDisplayNames                 []interface{} `json:"AcceptMessagesOnlyFromWithDisplayNames"`
	AcceptMessagesOnlyFromSendersOrMembersWithDisplayNames []interface{} `json:"AcceptMessagesOnlyFromSendersOrMembersWithDisplayNames"`
	AcceptMessagesOnlyFromDLMembersWithDisplayNames        []interface{} `json:"AcceptMessagesOnlyFromDLMembersWithDisplayNames"`
	ReportToManagerEnabled                                 bool          `json:"ReportToManagerEnabled"`
	ReportToOriginatorEnabled                              bool          `json:"ReportToOriginatorEnabled"`
	SendOofMessageToOriginatorEnabled                      bool          `json:"SendOofMessageToOriginatorEnabled"`
	Description                                            []interface{} `json:"Description"`
	BccBlocked                                             bool          `json:"BccBlocked"`
	AcceptMessagesOnlyFrom                                 []interface{} `json:"AcceptMessagesOnlyFrom"`
	AcceptMessagesOnlyFromDLMembers                        []interface{} `json:"AcceptMessagesOnlyFromDLMembers"`
	AcceptMessagesOnlyFromSendersOrMembers                 []interface{} `json:"AcceptMessagesOnlyFromSendersOrMembers"`
	AddressListMembership                                  []string      `json:"AddressListMembership"`
	AdministrativeUnits                                    []interface{} `json:"AdministrativeUnits"`
	Alias                                                  string        `json:"Alias"`
	ArbitrationMailbox                                     string        `json:"ArbitrationMailbox"`
	BypassModerationFromSendersOrMembers                   []interface{} `json:"BypassModerationFromSendersOrMembers"`
	OrganizationalUnit                                     string        `json:"OrganizationalUnit"`
	CustomAttribute1                                       string        `json:"CustomAttribute1"`
	CustomAttribute10                                      string        `json:"CustomAttribute10"`
	CustomAttribute11                                      string        `json:"CustomAttribute11"`
	CustomAttribute12                                      string        `json:"CustomAttribute12"`
	CustomAttribute13                                      string        `json:"CustomAttribute13"`
	CustomAttribute14                                      string        `json:"CustomAttribute14"`
	CustomAttribute15                                      string        `json:"CustomAttribute15"`
	CustomAttribute2                                       string        `json:"CustomAttribute2"`
	CustomAttribute3                                       string        `json:"CustomAttribute3"`
	CustomAttribute4                                       string        `json:"CustomAttribute4"`
	CustomAttribute5                                       string        `json:"CustomAttribute5"`
	CustomAttribute6                                       string        `json:"CustomAttribute6"`
	CustomAttribute7                                       string        `json:"CustomAttribute7"`
	CustomAttribute8                                       string        `json:"CustomAttribute8"`
	CustomAttribute9                                       string        `json:"CustomAttribute9"`
	ExtensionCustomAttribute1                              []interface{} `json:"ExtensionCustomAttribute1"`
	ExtensionCustomAttribute2                              []interface{} `json:"ExtensionCustomAttribute2"`
	ExtensionCustomAttribute3                              []interface{} `json:"ExtensionCustomAttribute3"`
	ExtensionCustomAttribute4                              []interface{} `json:"ExtensionCustomAttribute4"`
	ExtensionCustomAttribute5                              []interface{} `json:"ExtensionCustomAttribute5"`
	DisplayName                                            string        `json:"DisplayName"`
	EmailAddresses                                         []string      `json:"EmailAddresses"`
	GrantSendOnBehalfTo                                    []interface{} `json:"GrantSendOnBehalfTo"`
	ExternalDirectoryObjectID                              string        `json:"ExternalDirectoryObjectId"`
	HiddenFromAddressListsEnabled                          bool          `json:"HiddenFromAddressListsEnabled"`
	LastExchangeChangedTime                                interface{}   `json:"LastExchangeChangedTime"`
	LegacyExchangeDN                                       string        `json:"LegacyExchangeDN"`
	MaxSendSize                                            string        `json:"MaxSendSize"`
	MaxReceiveSize                                         string        `json:"MaxReceiveSize"`
	ModeratedBy                                            []interface{} `json:"ModeratedBy"`
	ModerationEnabled                                      bool          `json:"ModerationEnabled"`
	PoliciesIncluded                                       []interface{} `json:"PoliciesIncluded"`
	PoliciesExcluded                                       []string      `json:"PoliciesExcluded"`
	EmailAddressPolicyEnabled                              bool          `json:"EmailAddressPolicyEnabled"`
	PrimarySMTPAddress                                     string        `json:"PrimarySmtpAddress"`
	RecipientType                                          string        `json:"RecipientType"`
	RecipientTypeDetails                                   string        `json:"RecipientTypeDetails"`
	RejectMessagesFrom                                     []interface{} `json:"RejectMessagesFrom"`
	RejectMessagesFromDLMembers                            []interface{} `json:"RejectMessagesFromDLMembers"`
	RejectMessagesFromSendersOrMembers                     []interface{} `json:"RejectMessagesFromSendersOrMembers"`
	RequireSenderAuthenticationEnabled                     bool          `json:"RequireSenderAuthenticationEnabled"`
	SimpleDisplayName                                      string        `json:"SimpleDisplayName"`
	SendModerationNotifications                            string        `json:"SendModerationNotifications"`
	UMDtmfMap                                              []string      `json:"UMDtmfMap"`
	WindowsEmailAddress                                    string        `json:"WindowsEmailAddress"`
	MailTip                                                interface{}   `json:"MailTip"`
	MailTipTranslations                                    []interface{} `json:"MailTipTranslations"`
	Identity                                               string        `json:"Identity"`
	ID                                                     string        `json:"Id"`
	IsValid                                                bool          `json:"IsValid"`
	ExchangeVersion                                        string        `json:"ExchangeVersion"`
	Name                                                   string        `json:"Name"`
	DistinguishedName                                      string        `json:"DistinguishedName"`
	ObjectCategory                                         string        `json:"ObjectCategory"`
	ObjectClass                                            []string      `json:"ObjectClass"`
	WhenChanged                                            time.Time     `json:"WhenChanged"`
	WhenCreated                                            time.Time     `json:"WhenCreated"`
	WhenChangedUTC                                         time.Time     `json:"WhenChangedUTC"`
	WhenCreatedUTC                                         time.Time     `json:"WhenCreatedUTC"`
	ExchangeObjectID                                       string        `json:"ExchangeObjectId"`
	OrganizationalUnitRoot                                 string        `json:"OrganizationalUnitRoot"`
	OrganizationID                                         string        `json:"OrganizationId"`
	GUID                                                   string        `json:"Guid"`
	OriginatingServer                                      string        `json:"OriginatingServer"`
	ObjectState                                            string        `json:"ObjectState"`
}