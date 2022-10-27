package azure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/utils/env"
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
	envToken := env.GetString("AAS_AZURE_TOKEN", "")
	if envToken != "" {
		c.token = &BearerToken{token: envToken}
		return
	}

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

func (c *Client) GetGroups(prefix string) (*GroupsListResponse, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/groups", nil)
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$filter", fmt.Sprintf("startswith(displayName,'%s')", prefix))
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

func (c *Client) GetAdministrativeUnits() (*GetAdministrativeUnitsResponse, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/directory/administrativeUnits", nil)
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$filter", "startswith(displayName,'Team - Cloud Engineering')")
	req.URL.RawQuery = urlQueryValues.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GetAdministrativeUnitsResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) CreateAdministrativeUnitGroup(aUnitId string, rootId string) (*CreateAdministrativeUnitGroupResponse, error) {
	requestPayload := CreateAdministrativeUnitGroupRequest{
		OdataType:       "#Microsoft.Graph.Group",
		Description:     "[Automated] - aad-aws-sync",
		DisplayName:     fmt.Sprintf("CI_SSU_Cap - %s", rootId),
		MailNickname:    fmt.Sprintf("ci-ssu_cap_%s", rootId),
		GroupTypes:      []interface{}{},
		MailEnabled:     false,
		SecurityEnabled: true,
	}

	serialised, err := json.Marshal(requestPayload)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://graph.microsoft.com/v1.0/directory/administrativeUnits/%s/members", aUnitId), bytes.NewBuffer([]byte(serialised)))
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 201 {
		log.Println(string(rawData))
		log.Fatal(resp.StatusCode)
	}

	var payload *CreateAdministrativeUnitGroupResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) DeleteAdministrativeUnitGroup(aUnitId string, groupId string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://graph.microsoft.com/v1.0/directory/administrativeUnits/%s/members/%s", aUnitId, groupId), nil)
	if err != nil {
		return err
	}
	c.prepareHttpRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		log.Println(string(rawData))
		log.Fatal(resp.StatusCode)
	}

	return nil
}

func (c *Client) AddGroupMember(groupId string, upn string) error {
	requestPayload := AddGroupMemberRequest{
		OdataId: fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s", upn),
	}

	serialised, err := json.Marshal(requestPayload)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/members/$ref", groupId), bytes.NewBuffer([]byte(serialised)))
	if err != nil {
		return err
	}
	c.prepareHttpRequest(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		if resp.StatusCode == 404 {
			log.Printf("User %s not found, skipping.", upn)
			return nil
		}

		if resp.StatusCode == 403 {
			log.Println("Response returned with unexpected 403. Skipping entry.")
			return nil
		}

		log.Fatal(resp.StatusCode)
	}

	return nil
}

func (c *Client) GetAdministrativeUnitMembers(id string) (*GetAdministrativeUnitMembersResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/directory/administrativeUnits/%s/members", id), nil)
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

	var payload *GetAdministrativeUnitMembersResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	nextLink := payload.OdataNextLink

	for nextLink != "" {
		fmt.Println("Fetching additional groups")
		req, err := http.NewRequest("GET", nextLink, nil)
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

		var buffer *GetAdministrativeUnitMembersResponse

		err = json.Unmarshal(rawData, &buffer)
		if err != nil {
			return nil, err
		}

		nextLink = buffer.OdataNextLink

		payload.Value = append(payload.Value, buffer.Value...)
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

func (c *Client) GetApplicationRoles(appId string) (*GetApplicationRolesResponse, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/applications", nil)
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$filter", fmt.Sprintf("appId eq '%s'", appId))
	urlQueryValues.Set("$select", "displayName, appId, appRoles")
	req.URL.RawQuery = urlQueryValues.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GetApplicationRolesResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) GetAssignmentsForApplication(appObjectId string) (*GetAssignmentsForApplicationResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/beta/servicePrincipals/%s/appRoleAssignedTo", appObjectId), nil)
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

	var payload *GetAssignmentsForApplicationResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	nextLink := payload.OdataNextLink

	for nextLink != "" {
		fmt.Println("Fetching additional assignments")
		req, err := http.NewRequest("GET", nextLink, nil)
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

		var buffer *GetAssignmentsForApplicationResponse

		err = json.Unmarshal(rawData, &buffer)
		if err != nil {
			return nil, err
		}

		nextLink = buffer.OdataNextLink

		payload.Value = append(payload.Value, buffer.Value...)
	}

	return payload, nil
}

func (c *Client) AssignGroupToApplication(appObjectId string, groupId string, roleId string) (*AssignGroupToApplicationResponse, error) {
	requestPayload := AssignGroupToApplicationRequest{
		PrincipalID: groupId,
		ResourceID:  appObjectId,
		AppRoleID:   roleId,
	}

	serialised, err := json.Marshal(requestPayload)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/appRoleAssignments", groupId), bytes.NewBuffer([]byte(serialised)))
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 201 {
		log.Println(string(rawData))
		log.Fatal(resp.StatusCode)
	}

	var payload *AssignGroupToApplicationResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) UnassignGroupFromApplication(groupId string, assignmentId string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/appRoleAssignments/%s", groupId, assignmentId), nil)
	if err != nil {
		return nil
	}
	c.prepareHttpRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	if resp.StatusCode != 204 {
		log.Println(string(rawData))
		log.Fatal(resp.StatusCode)
	}

	return nil
}

func NewAzureClient(conf Config) *Client {
	payload := &Client{
		httpClient: http.DefaultClient,
		config:     conf,
	}
	return payload
}
