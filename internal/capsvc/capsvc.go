package capsvc

import (
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
	Host         string
	TenantId     string `json:"tenantId"`
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Scope        string `json:"scope"`
}

func (c *Client) prepareHttpRequest(h *http.Request) error {
	err := c.RefreshAuth()
	if err != nil {
		return err
	}

	h.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tokenClient.Token.GetToken()))
	h.Header.Set("User-Agent", "aad-aws-sync - github.com/dfds/aad-aws-sync")

	return nil
}

func (c *Client) GetCapabilities() (*GetCapabilitiesResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/capabilities", c.config.Host), nil)
	if err != nil {
		return nil, err
	}
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("response returned unexpected status code: %d", resp.StatusCode)
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GetCapabilitiesResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) RefreshAuth() error {
	envToken := env.GetString("AAS_CAPSVC_TOKEN", "")
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
	reqPayload.Set("scope", c.config.Scope)
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

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("response returned unexpected status code: %d", resp.StatusCode)
	}

	var tokenResponse *util.RefreshAuthResponse

	err = json.Unmarshal(rawData, &tokenResponse)
	if err != nil {
		return nil, err
	}

	return tokenResponse, nil
}

func NewCapSvcClient(conf Config) *Client {
	payload := &Client{
		httpClient: http.DefaultClient,
		config:     conf,
	}
	payload.tokenClient = util.NewTokenClient(payload.getNewToken)
	return payload
}
