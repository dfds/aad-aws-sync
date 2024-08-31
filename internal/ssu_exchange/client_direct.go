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

func (c *ClientO365UnofficialApi) o365BaseUrl() string {
	return fmt.Sprintf("https://outlook.office365.com/adminapi/beta/%s/InvokeCommand", c.config.TenantId)
}

func (c *ClientO365UnofficialApi) GetAliases(ctx context.Context) ([]GetAliasesResponse, error) {
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "Get-DistributionGroup",
			Parameters: direct.CmdletInputParameters{
				Filter: "Name -like 'CI_SSU_Ex*'",
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

	err = c.prepareJsonRequest(req)
	if err != nil {
		return nil, err
	}

	rf := NewRequestFuncs()
	rf.PostResponse = func(req *http.Request, resp *http.Response) error {
		if resp.StatusCode != 200 {
			return fmt.Errorf("response returned unexpected status code: %d", resp.StatusCode)
		}
		return nil
	}
	payload, err := DoRequest[direct.O365ResponseWrapper[GetAliasesResponse]](c, req, rf)
	if err != nil {
		return nil, err
	}

	return payload.Value, nil
}

func (c *ClientO365UnofficialApi) CreateAlias(ctx context.Context, alias string, displayName string, members []string) error {
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "New-DistributionGroup",
			Parameters: direct.CmdletInputParameters{
				Name:                               fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, displayName),
				Alias:                              fmt.Sprintf("%s.ssu", alias),
				PrimarySmtpAddress:                 fmt.Sprintf("%s%s", alias, c.config.EmailSuffix),
				MemberJoinRestriction:              "Closed",
				RequireSenderAuthenticationEnabled: false,
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
	rf.PostResponse = func(req *http.Request, resp *http.Response) error {
		if resp.StatusCode != 200 {
			return fmt.Errorf("response returned unexpected status code: %d", resp.StatusCode)
		}
		return nil
	}
	err = DoRequestWithoutDeserialise(c, req, rf)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientO365UnofficialApi) RemoveAlias(ctx context.Context, alias string) error {
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "Remove-DistributionGroup",
			Parameters: direct.CmdletInputParameters{
				Identity: fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, alias),
				Confirm:  false,
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
	rf.PostResponse = func(req *http.Request, resp *http.Response) error {
		if resp.StatusCode != 200 {
			return fmt.Errorf("response returned unexpected status code: %d", resp.StatusCode)
		}
		return nil
	}
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
	rf.PostResponse = func(req *http.Request, resp *http.Response) error {
		if resp.StatusCode != 200 {
			return fmt.Errorf("response returned unexpected status code: %d", resp.StatusCode)
		}
		return nil
	}
	err = DoRequestWithoutDeserialise(c, req, rf)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientO365UnofficialApi) RemoveDistributionGroupMember(ctx context.Context, displayName string, memberEmail string) error {
	reqPayload := direct.O365BaseRequest{
		CmdletInput: direct.RequestCmdletInput{
			CmdletName: "Remove-DistributionGroupMember",
			Parameters: direct.CmdletInputParameters{
				Identity: fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, displayName),
				Member:   memberEmail,
				Confirm:  false,
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
	rf.PostResponse = func(req *http.Request, resp *http.Response) error {
		if resp.StatusCode != 200 {
			return fmt.Errorf("response returned unexpected status code: %d", resp.StatusCode)
		}
		return nil
	}
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
