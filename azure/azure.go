package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	config     Config
	token      *BearerToken
}

type Config struct {
	TenantId     string `json:"tenantId"`
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type BearerToken struct {
	token     string
	expiresIn int64
}

type RefreshAuthResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	ExtExpiresIn int64  `json:"ext_expires_in"`
	AccessToken  string `json:"access_token"`
}

func (b *BearerToken) IsExpired() bool {
	if b.token == "" {
		return true
	}

	currentTime := time.Now()
	tokenExpirationTime := time.Unix(b.expiresIn, 0)
	return currentTime.After(tokenExpirationTime)
}

func (c *Client) refreshAuth() {
	if c.token != nil {
		if !c.token.IsExpired() {
			//fmt.Println("Token has not expired, reusing token from cache")
			return
		}
	}

	reqPayload := url.Values{}
	reqPayload.Set("client_id", c.config.ClientId)
	reqPayload.Set("grant_type", "client_credentials")
	reqPayload.Set("scope", "https://graph.microsoft.com/.default")
	reqPayload.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequest("POST", fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.config.TenantId), strings.NewReader(reqPayload.Encode()))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("Unexpected response from token request.\nStatus code: %d\nContent: %s\n", resp.StatusCode, string(rawData))
	}

	var tokenResponse RefreshAuthResponse

	err = json.Unmarshal(rawData, &tokenResponse)
	if err != nil {
		log.Fatal(err)
	}

	currentTime := time.Now()
	c.token = &BearerToken{}
	c.token.expiresIn = currentTime.Unix() + tokenResponse.ExpiresIn
	c.token.token = tokenResponse.AccessToken
}

func (c *Client) prepareHttpRequest(h *http.Request) {
	c.refreshAuth()

	h.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token.token))
}

func (c *Client) GetGroups() (*GroupsListResponse, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/groups", nil)
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$filter", "startswith(displayName,'Cloud Infrastructure - SSU - Capability')")
	req.URL.RawQuery = urlQueryValues.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GroupsListResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) GetGroupMembers(id string) (*GroupMembers, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/members", id), nil)
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GroupMembers

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func NewAzureClient(conf Config) *Client {
	payload := &Client{
		httpClient: http.DefaultClient,
		config:     conf,
	}
	return payload
}
