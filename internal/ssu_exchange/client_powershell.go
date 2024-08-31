package ssu_exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"io"
	"k8s.io/utils/env"
	"net/http"
	"net/url"
	"strings"
)

type ClientPowershellWrapper struct {
	httpClient  *http.Client
	tokenClient *util.TokenClient
	config      Config
}

func (c *ClientPowershellWrapper) GetAliases(ctx context.Context) ([]GetAliasesResponse, error) {
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

func (c *ClientPowershellWrapper) CreateAlias(ctx context.Context, alias string, displayName string, members []string) error {
	serialised, err := json.Marshal(struct {
		Alias       string   `json:"alias"`
		ManagedBy   string   `json:"managedBy"`
		DisplayName string   `json:"displayName"`
		Members     []string `json:"members,omitempty"`
	}{
		Alias:       alias,
		ManagedBy:   c.config.ManagedBy,
		DisplayName: displayName,
		Members:     members,
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

func (c *ClientPowershellWrapper) RemoveAlias(ctx context.Context, alias string) error {
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

func (c *ClientPowershellWrapper) AddDistributionGroupMember(ctx context.Context, displayName string, memberEmail string) error {
	serialised, err := json.Marshal(struct {
		DisplayName string `json:"displayName"`
		Member      string `json:"member"`
	}{
		DisplayName: displayName,
		Member:      memberEmail,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/AddDistributionGroupMember", c.config.BaseUrl)

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

func (c *ClientPowershellWrapper) RemoveDistributionGroupMember(ctx context.Context, displayName string, memberEmail string) error {
	serialised, err := json.Marshal(struct {
		DisplayName string `json:"displayName"`
		Member      string `json:"member"`
	}{
		DisplayName: displayName,
		Member:      memberEmail,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/RemoveDistributionGroupMember", c.config.BaseUrl)

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

func (c *ClientPowershellWrapper) RefreshAuth() error {
	envToken := env.GetString("AAS_AZURE_TOKEN", "")
	if envToken != "" {
		c.tokenClient.Token = util.NewBearerToken(envToken)
		return nil
	}

	err := c.tokenClient.RefreshAuth()
	return err
}

func (c *ClientPowershellWrapper) getNewToken() (*util.RefreshAuthResponse, error) {
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

func (c *ClientPowershellWrapper) prepareHttpRequest(req *http.Request) error {
	err := c.RefreshAuth()
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tokenClient.Token.GetToken()))
	req.Header.Set("User-Agent", "aad-aws-sync - github.com/dfds/aad-aws-sync")
	return nil
}

func (c *ClientPowershellWrapper) prepareJsonRequest(req *http.Request) error {
	err := c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (c *ClientPowershellWrapper) GetHttpClient() *http.Client {
	return c.httpClient
}