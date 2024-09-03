package ssu_exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/ssu_exchange/direct"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"io"
	"net/http"
)

type IClient interface {
	GetAliases(ctx context.Context) ([]GetAliasesResponse, error)
	CreateAlias(ctx context.Context, alias string, displayName string, members []string) error
	RemoveAlias(ctx context.Context, alias string) error
	UpdateAlias(ctx context.Context, alias string, params direct.CmdletInputParameters) error
	AddDistributionGroupMember(ctx context.Context, displayName string, memberEmail string) error
	RemoveDistributionGroupMember(ctx context.Context, displayName string, memberEmail string) error
	RefreshAuth() error
	GetHttpClient() *http.Client
}

type Config struct {
	TenantId     string `json:"tenantId"`
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	BaseUrl      string `json:"baseUrl"`
	ManagedBy    string `json:"managedBy"`
	EmailSuffix  string `json:"emailSuffix"`
}

func DoRequest[T any](client IClient, req *http.Request, rf *RequestFuncs) (*T, error) {
	if rf == nil {
		rf = NewRequestFuncs()
	}
	resp, _, err := DoRequestWithResp[T](client, req, rf)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func DoRequestWithoutDeserialise(client IClient, req *http.Request, rf *RequestFuncs) error {
	err := rf.PreResponse(req)
	if err != nil {
		return err
	}

	resp, err := client.GetHttpClient().Do(req)
	if err != nil {
		return err
	}

	err = rf.PostResponse(req, resp)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func DoRequestWithResp[T any](client IClient, req *http.Request, rf *RequestFuncs) (*T, *http.Response, error) {
	err := rf.PreResponse(req)
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.GetHttpClient().Do(req)
	if err != nil {
		return nil, nil, err
	}

	err = rf.PostResponse(req, resp)
	if err != nil {
		return nil, resp, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}

	var payload *T

	err = rf.PreDeserialise(req, resp)
	if err != nil {
		return nil, resp, err
	}

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, resp, err
	}

	err = rf.PostDeserialise(req, resp, payload)
	if err != nil {
		return payload, resp, err
	}

	return payload, resp, nil
}

type RequestFuncs struct {
	PreResponse     func(req *http.Request) error
	PostResponse    func(req *http.Request, resp *http.Response) error
	PreDeserialise  func(req *http.Request, resp *http.Response) error
	PostDeserialise func(req *http.Request, resp *http.Response, data interface{}) error
}

func NewRequestFuncs() *RequestFuncs {
	rf := &RequestFuncs{
		PreResponse: func(req *http.Request) error {
			return nil
		},
		PostResponse: func(req *http.Request, resp *http.Response) error {
			return nil
		},
		PreDeserialise: func(req *http.Request, resp *http.Response) error {
			return nil
		},
		PostDeserialise: func(req *http.Request, resp *http.Response, data interface{}) error {
			return nil
		},
	}

	return rf
}

func (c *ClientPowershellWrapper) HasTokenExpired() bool {
	return c.tokenClient.Token.IsExpired()
}

func NewSsuExchangeClientPowershellWrapper(conf Config) IClient {
	payload := &ClientPowershellWrapper{
		httpClient: http.DefaultClient,
		config:     conf,
	}

	payload.tokenClient = util.NewTokenClient(payload.getNewToken)

	return payload
}

func NewSsuExchangeClientO365UnofficialApi(conf Config) IClient {
	payload := &ClientO365UnofficialApi{
		httpClient: http.DefaultClient,
		config:     conf,
	}

	payload.tokenClient = util.NewTokenClient(payload.getNewToken)

	return payload
}

const AZURE_CAPABILITY_GROUP_PREFIX = "CI_SSU_Ex -"

func GenerateExchangeDistributionGroupDisplayName(name string) string {
	return fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, name)
}
