package ssu_exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"go.dfds.cloud/aad-aws-sync/internal/util"
	"k8s.io/utils/env"
)

type Client struct {
	httpClient  *http.Client
	tokenClient *util.TokenClient
	config      Config
}

type Config struct {
	TenantId     string `json:"tenantId"`
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	BaseUrl      string `json:"baseUrl"`
	ManagedBy    string `json:"managedBy"`
}

func (c *Client) GetAliases(ctx context.Context) ([]GetAliasesResponse, error) {
	url := fmt.Sprintf("%s/GetDistributionGroups", c.config.BaseUrl)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	err = c.prepareHttpRequest(req)
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
	payload, err := DoRequest[[]GetAliasesResponse](c, req, rf)
	if err != nil {
		return nil, err
	}

	return *payload, nil
}

func (c *Client) CreateAlias(ctx context.Context, alias string) error {
	serialised, err := json.Marshal(struct {
		Alias     string `json:"alias"`
		ManagedBy string `json:"managedBy"`
	}{
		Alias:     alias,
		ManagedBy: c.config.ManagedBy,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/CreateDistributionGroup", c.config.BaseUrl)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(serialised))
	if err != nil {
		return err
	}

	err = c.prepareHttpRequest(req)
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

func (c *Client) RemoveAlias(ctx context.Context, alias string) error {
	serialised, err := json.Marshal(struct {
		Alias string `json:"alias"`
	}{
		Alias: alias,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/DeleteDistributionGroup", c.config.BaseUrl)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(serialised))
	if err != nil {
		return err
	}

	err = c.prepareHttpRequest(req)
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

func (c *Client) RefreshAuth() error {
	envToken := env.GetString("AAS_AZURE_TOKEN", "")
	if envToken != "" {
		c.tokenClient.Token = util.NewBearerToken(envToken)
		return nil
	}

	err := c.tokenClient.RefreshAuth()
	return err
}

func (c *Client) getNewToken() (*util.RefreshAuthResponse, error) {
	reqPayload := url.Values{}
	reqPayload.Set("client_id", c.config.ClientId)
	reqPayload.Set("grant_type", "client_credentials")
	reqPayload.Set("scope", "ae87d685-cfcd-4019-8d4b-a82cc27fd2bc/.default")
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

func (c *Client) prepareHttpRequest(req *http.Request) error {
	err := c.RefreshAuth()
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tokenClient.Token.GetToken()))
	req.Header.Set("User-Agent", "aad-aws-sync - github.com/dfds/aad-aws-sync")
	return nil
}

func (c *Client) prepareJsonRequest(req *http.Request) error {
	err := c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	return nil
}

func DoRequest[T any](client *Client, req *http.Request, rf *RequestFuncs) (*T, error) {
	if rf == nil {
		rf = NewRequestFuncs()
	}
	resp, _, err := DoRequestWithResp[T](client, req, rf)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func DoRequestWithoutDeserialise(client *Client, req *http.Request, rf *RequestFuncs) error {
	err := rf.PreResponse(req)
	if err != nil {
		return err
	}

	resp, err := client.httpClient.Do(req)
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

func DoRequestWithResp[T any](client *Client, req *http.Request, rf *RequestFuncs) (*T, *http.Response, error) {
	err := rf.PreResponse(req)
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.httpClient.Do(req)
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

func (c *Client) HasTokenExpired() bool {
	return c.tokenClient.Token.IsExpired()
}

func NewSsuExchangeClient(conf Config) *Client {
	payload := &Client{
		httpClient: http.DefaultClient,
		config:     conf,
	}

	payload.tokenClient = util.NewTokenClient(payload.getNewToken)

	return payload
}

const AZURE_CAPABILITY_GROUP_PREFIX = "CI_SSU_Exchange -"

func GenerateExchangeDistributionGroupDisplayName(name string) string {
	return fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, name)
}
