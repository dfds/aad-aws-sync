package capsvc

import (
	"errors"
	"strings"
)

type GetCapabilitiesResponse struct {
	Items []*GetCapabilitiesResponseContextCapability `json:"items"`
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
	ID          string                                           `json:"id"`
	Name        string                                           `json:"name"`
	RootID      string                                           `json:"rootId"`
	Description string                                           `json:"description"`
	Members     []GetCapabilitiesResponseContextCapabilityMember `json:"members"`
	Contexts    []*GetCapabilitiesResponseContext                `json:"contexts,omitempty"`
}

type GetCapabilitiesResponseContextCapabilityMember struct {
	Email string `json:"email"`
	// UserID is the member's authoritative identifier from selfservice-api. For
	// regular users this is their UPN, which can differ from their email address.
	UserID string `json:"userId"`
}

// HasMember reports whether the capability contains a member matching value,
// which may be either an email or a UPN. A user's UPN and email can differ, so
// both the member's email and userId (UPN) are compared to avoid false negatives
// that would wrongly flag an Azure AD member as stale and remove them.
func (g *GetCapabilitiesResponseContextCapability) HasMember(value string) bool {
	for _, member := range g.Members {
		if strings.EqualFold(member.Email, value) {
			return true
		}
		if member.UserID != "" && strings.EqualFold(member.UserID, value) {
			return true
		}
	}

	return false
}

type GetCapabilitiesResponseContext struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AwsAccountID string `json:"awsAccountId"`
	AwsRoleArn   string `json:"awsRoleArn"`
	AwsRoleEmail string `json:"awsRoleEmail"`
}
