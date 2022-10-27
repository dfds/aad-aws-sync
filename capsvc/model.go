package capsvc

import "errors"

type GetCapabilitiesResponse struct {
	Items []GetCapabilitiesResponseContextCapability `json:"items"`
}

func (g *GetCapabilitiesResponseContextCapability) GetContext() (*GetCapabilitiesResponseContext, error) {
	if len(g.Contexts) > 0 {
		if g.Contexts[0].AwsAccountID == "" {
			return g.Contexts[0], errors.New("capability has a Context, but no AWS account associated with the aforementioned Context")
		}
		return g.Contexts[0], nil
	} else {
		return nil, errors.New("capability doesn't have a Context")
	}
}

type GetCapabilitiesResponseContextCapability struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RootID      string `json:"rootId"`
	Description string `json:"description"`
	Members     []struct {
		Email string `json:"email"`
	} `json:"members"`
	Contexts []*GetCapabilitiesResponseContext `json:"contexts,omitempty"`
}

type GetCapabilitiesResponseContext struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AwsAccountID string `json:"awsAccountId"`
	AwsRoleArn   string `json:"awsRoleArn"`
	AwsRoleEmail string `json:"awsRoleEmail"`
}
