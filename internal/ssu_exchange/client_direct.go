package ssu_exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/ssu_exchange/direct"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"io"
	"k8s.io/utils/env"
	"net/http"
	"net/url"
	"strings"
)

type ClientO365UnofficialApi struct {
	httpClient  *http.Client
	tokenClient *util.TokenClient
	config      Config
}

// o365AnchorMailbox is the well-known arbitration/system mailbox the adminapi
// InvokeCommand endpoint uses to route org-level cmdlets. The endpoint returns
// a bare 403 for write cmdlets when this header is absent, so every request
// sets it (see prepareHttpRequest).
const o365AnchorMailbox = "UPN:SystemMailbox{bb558c35-97f1-4cb9-8ff7-d53741dc928c}@dfds.onmicrosoft.com"

// newStatusCodePostResponse returns a PostResponse hook that, on a non-200
// status, reads the response body and returns an error annotated with the
// cmdlet name, the target recipient it acted on, the status code, and the
// body. The O365 unofficial API returns the actual failure reason (e.g. the
// denied recipient/cmdlet on a 403) in the body, so surfacing it together
// with the target is what makes these errors diagnosable.
func newStatusCodePostResponse(cmdletName string, params direct.CmdletInputParameters) func(req *http.Request, resp *http.Response) error {
	target := formatCmdletTarget(params)
	return func(req *http.Request, resp *http.Response) error {
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cmdlet %s for target %q returned unexpected status code: %d, body: %s%s", cmdletName, target, resp.StatusCode, strings.TrimSpace(string(body)), formatDiagnosticHeaders(resp))
	}
}

// formatDiagnosticHeaders surfaces the response headers Exchange Online uses to
// explain auth/RBAC failures. The adminapi often returns an empty body on a
// 403, putting the real reason (e.g. a denied management role) in
// x-ms-diagnostics instead.
func formatDiagnosticHeaders(resp *http.Response) string {
	var parts []string
	for _, h := range []string{"X-Ms-Diagnostics", "Www-Authenticate", "X-Calculatedbetarget", "X-Backendhttpstatus"} {
		if v := resp.Header.Get(h); v != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", h, v))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ", headers: {" + strings.Join(parts, "; ") + "}"
}

// formatCmdletTarget builds a human-readable identifier for the recipient a
// cmdlet acts on, so a failure names the distribution group (and member, for
// membership cmdlets) rather than just the cmdlet.
func formatCmdletTarget(p direct.CmdletInputParameters) string {
	identity := p.Identity
	if identity == "" {
		identity = p.Name // New-DistributionGroup uses Name rather than Identity
	}
	if identity == "" {
		identity = p.Filter // read cmdlets (Get-DistributionGroup) target by Filter
	}
	if p.Member != "" {
		return fmt.Sprintf("%s member %s", identity, p.Member)
	}
	return identity
}

func (c *ClientO365UnofficialApi) o365BaseUrl() string {
	return fmt.Sprintf("https://outlook.office365.com/adminapi/beta/%s/InvokeCommand", c.config.TenantId)
}

func (c *ClientO365UnofficialApi) GetAliases(ctx context.Context) ([]GetAliasesResponse, error) {
	var skipToken *string = nil
	var data []GetAliasesResponse
	for {
		reqPayload := direct.O365BaseRequest{
			CmdletInput: direct.RequestCmdletInput{
				CmdletName: "Get-DistributionGroup",
				Parameters: direct.CmdletInputParameters{
					Filter:     "Name -like 'CI_SSU_Ex*'",
					ResultSize: "unlimited",
				},
			},
		}

		serialisedReqPayload, err := json.Marshal(reqPayload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.o365BaseUrl(), bytes.NewBuffer(serialisedReqPayload))
		if err != nil {
			return nil, err
		}
		if skipToken != nil {
			req.URL, err = url.Parse(*skipToken)
			if err != nil {
				return nil, err
			}
		}

		err = c.prepareJsonRequest(req)
		if err != nil {
			return nil, err
		}

		rf := NewRequestFuncs()
		rf.PostResponse = newStatusCodePostResponse(reqPayload.CmdletInput.CmdletName, reqPayload.CmdletInput.Parameters)
		payload, err := DoRequest[direct.O365ResponseWrapper[GetAliasesResponse]](c, req, rf)
		if err != nil {
			return nil, err
		}

		data = append(data, payload.Value...)

		if payload.OdataNextLink != nil {
			skipToken = payload.OdataNextLink
		} else {
			break
		}
	}

	return data, nil
}

func (c *ClientO365UnofficialApi) CreateAlias(ctx context.Context, alias string, displayName string, members []string) error {
	requireSenderAuthenticationEnabled := false
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "New-DistributionGroup",
			Parameters: direct.CmdletInputParameters{
				Name:                               fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, displayName),
				Alias:                              fmt.Sprintf("%s.ssu", alias),
				PrimarySmtpAddress:                 fmt.Sprintf("%s%s", alias, c.config.EmailSuffix),
				MemberJoinRestriction:              "Closed",
				RequireSenderAuthenticationEnabled: &requireSenderAuthenticationEnabled,
				Members:                            members,
				ManagedBy:                          c.config.ManagedBy,
			},
		},
	}

	serialisedReqPayload, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.o365BaseUrl(), bytes.NewBuffer(serialisedReqPayload))
	if err != nil {
		return err
	}

	err = c.prepareJsonRequest(req)
	if err != nil {
		return err
	}

	rf := NewRequestFuncs()
	rf.PostResponse = newStatusCodePostResponse(reqPayload.CmdletInput.CmdletName, reqPayload.CmdletInput.Parameters)
	err = DoRequestWithoutDeserialise(c, req, rf)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientO365UnofficialApi) RemoveAlias(ctx context.Context, alias string) error {
	confirm := false
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "Remove-DistributionGroup",
			Parameters: direct.CmdletInputParameters{
				Identity: fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, alias),
				Confirm:  &confirm,
			},
		},
	}

	serialisedReqPayload, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.o365BaseUrl(), bytes.NewBuffer(serialisedReqPayload))
	if err != nil {
		return err
	}

	err = c.prepareJsonRequest(req)
	if err != nil {
		return err
	}

	rf := NewRequestFuncs()
	rf.PostResponse = newStatusCodePostResponse(reqPayload.CmdletInput.CmdletName, reqPayload.CmdletInput.Parameters)
	err = DoRequestWithoutDeserialise(c, req, rf)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientO365UnofficialApi) UpdateAlias(ctx context.Context, alias string, params direct.CmdletInputParameters) error {
	params.Identity = alias
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "Set-DistributionGroup",
			Parameters: params,
		},
	}

	serialisedReqPayload, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.o365BaseUrl(), bytes.NewBuffer(serialisedReqPayload))
	if err != nil {
		return err
	}

	err = c.prepareJsonRequest(req)
	if err != nil {
		return err
	}

	rf := NewRequestFuncs()
	rf.PostResponse = newStatusCodePostResponse(reqPayload.CmdletInput.CmdletName, reqPayload.CmdletInput.Parameters)
	err = DoRequestWithoutDeserialise(c, req, rf)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientO365UnofficialApi) AddDistributionGroupMember(ctx context.Context, displayName string, memberEmail string) error {
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "Add-DistributionGroupMember",
			Parameters: direct.CmdletInputParameters{
				Identity: fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, displayName),
				Member:   memberEmail,
			},
		},
	}

	serialisedReqPayload, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.o365BaseUrl(), bytes.NewBuffer(serialisedReqPayload))
	if err != nil {
		return err
	}

	err = c.prepareJsonRequest(req)
	if err != nil {
		return err
	}

	rf := NewRequestFuncs()
	rf.PostResponse = newStatusCodePostResponse(reqPayload.CmdletInput.CmdletName, reqPayload.CmdletInput.Parameters)
	err = DoRequestWithoutDeserialise(c, req, rf)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientO365UnofficialApi) RemoveDistributionGroupMember(ctx context.Context, displayName string, memberEmail string) error {
	confirm := false
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "Remove-DistributionGroupMember",
			Parameters: direct.CmdletInputParameters{
				Identity: fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, displayName),
				Member:   memberEmail,
				Confirm:  &confirm,
			},
		},
	}

	serialisedReqPayload, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.o365BaseUrl(), bytes.NewBuffer(serialisedReqPayload))
	if err != nil {
		return err
	}

	err = c.prepareJsonRequest(req)
	if err != nil {
		return err
	}

	rf := NewRequestFuncs()
	rf.PostResponse = newStatusCodePostResponse(reqPayload.CmdletInput.CmdletName, reqPayload.CmdletInput.Parameters)
	err = DoRequestWithoutDeserialise(c, req, rf)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientO365UnofficialApi) RefreshAuth() error {
	envToken := env.GetString("AAS_AZURE_TOKEN", "")
	if envToken != "" {
		c.tokenClient.Token = util.NewBearerToken(envToken)
		return nil
	}

	err := c.tokenClient.RefreshAuth()
	return err
}

func (c *ClientO365UnofficialApi) getNewToken() (*util.RefreshAuthResponse, error) {
	reqPayload := url.Values{}
	reqPayload.Set("client_id", c.config.ClientId)
	reqPayload.Set("grant_type", "client_credentials")
	reqPayload.Set("scope", "https://outlook.office365.com/.default")
	reqPayload.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequest("POST", fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.config.TenantId), strings.NewReader(reqPayload.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, err
	}

	var tokenResponse *util.RefreshAuthResponse

	err = json.Unmarshal(rawData, &tokenResponse)
	if err != nil {
		return nil, err
	}

	return tokenResponse, nil
}

func (c *ClientO365UnofficialApi) prepareHttpRequest(req *http.Request) error {
	err := c.RefreshAuth()
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tokenClient.Token.GetToken()))
	req.Header.Set("User-Agent", "aad-aws-sync - github.com/dfds/aad-aws-sync")
	req.Header.Set("X-AnchorMailbox", o365AnchorMailbox)
	return nil
}

func (c *ClientO365UnofficialApi) prepareJsonRequest(req *http.Request) error {
	err := c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (c *ClientO365UnofficialApi) GetHttpClient() *http.Client {
	return c.httpClient
}

func ConvertGetAliasesRawResponseToGetAliasesResponse(response *direct.GetAliasesRawResponse) []GetAliasesResponse {
	var payload []GetAliasesResponse

	for _, obj := range response.Objs.Obj.Lst.Obj {
		newAlias := GetAliasesResponse{
			DisplayName: obj.ToString,
		}

		payload = append(payload, newAlias)
	}

	return payload
}
