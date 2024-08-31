package direct

import "time"

type O365BaseRequest struct {
	CmdletInput RequestCmdletInput `json:"CmdletInput"`
}

type RequestCmdletInput struct {
	CmdletName string                `json:"CmdletName"`
	Parameters CmdletInputParameters `json:"Parameters"`
}

type CmdletInputParameters struct {
	Filter                             string   `json:"Filter,omitempty"`
	Name                               string   `json:"Name,omitempty"`
	Alias                              string   `json:"Alias,omitempty"`
	PrimarySmtpAddress                 string   `json:"PrimarySmtpAddress,omitempty"`
	MemberJoinRestriction              string   `json:"MemberJoinRestriction,omitempty"`
	RequireSenderAuthenticationEnabled bool     `json:"RequireSenderAuthenticationEnabled,omitempty"`
	Members                            []string `json:"Members,omitempty"`
	Member                             string   `json:"Member,omitempty"`
	ManagedBy                          string   `json:"ManagedBy,omitempty"`
	Identity                           string   `json:"Identity,omitempty"`
	Confirm                            bool     `json:"Confirm,omitempty"`
}

type O365ResponseWrapper[T any] struct {
	OdataContext              string        `json:"@odata.context"`
	AdminapiWarningsOdataType string        `json:"adminapi.warnings@odata.type"`
	AdminapiWarnings          []interface{} `json:"@adminapi.warnings"`
	Value                     []T           `json:"value"`
}

type GetAliasesRawResponse struct {
	Objs struct {
		Version string `json:"-Version"`
		Xmlns   string `json:"-xmlns"`
		Obj     struct {
			Lst struct {
				Obj []struct {
					RefID string `json:"-RefId"`
					Tn    struct {
						RefID string   `json:"-RefId"`
						T     []string `json:"T"`
					} `json:"TN"`
					ToString string `json:"ToString"`
					Props    struct {
						S []struct {
							Content string `json:"#content,omitempty"`
							N       string `json:"-N"`
						} `json:"S"`
						B []struct {
							Content string `json:"#content"`
							N       string `json:"-N"`
						} `json:"B"`
						Obj []struct {
							N     string `json:"-N"`
							RefID string `json:"-RefId"`
							Tn    struct {
								RefID string   `json:"-RefId"`
								T     []string `json:"T"`
							} `json:"TN,omitempty"`
							Lst struct {
								S string `json:"S"`
							} `json:"LST"`
							TNRef struct {
								RefID string `json:"-RefId"`
							} `json:"TNRef,omitempty"`
						} `json:"Obj"`
						Nil []struct {
							N string `json:"-N"`
						} `json:"Nil"`
						Dt []struct {
							Content time.Time `json:"#content"`
							N       string    `json:"-N"`
						} `json:"DT"`
						G []struct {
							Content string `json:"#content"`
							N       string `json:"-N"`
						} `json:"G"`
					} `json:"Props"`
				} `json:"Obj"`
			} `json:"LST"`
		} `json:"Obj"`
	} `json:"Objs"`
}
