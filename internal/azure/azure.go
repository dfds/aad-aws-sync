package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"io"
	"k8s.io/utils/env"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// TODO look into using: https://github.com/microsoftgraph/msgraph-sdk-go

type Client struct {
	httpClient  *http.Client
	tokenClient *util.TokenClient
	config      Config
}

type Config struct {
	TenantId     string `json:"tenantId"`
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

func (c *Client) RefreshAuth() {
	envToken := env.GetString("AAS_AZURE_TOKEN", "")
	if envToken != "" {
		c.tokenClient.Token = util.NewBearerToken(envToken)
		return
	}

	c.tokenClient.RefreshAuth()
}

func (c *Client) getNewToken() (*util.RefreshAuthResponse, error) {
	reqPayload := url.Values{}
	reqPayload.Set("client_id", c.config.ClientId)
	reqPayload.Set("grant_type", "client_credentials")
	reqPayload.Set("scope", "https://graph.microsoft.com/.default")
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

func (c *Client) prepareHttpRequest(req *http.Request) {
	c.RefreshAuth()

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tokenClient.Token.GetToken()))
	req.Header.Set("User-Agent", "aad-aws-sync - github.com/dfds/aad-aws-sync")
}

func (c *Client) prepareJsonRequest(req *http.Request) {
	c.prepareHttpRequest(req)
	req.Header.Set("Content-Type", "application/json")
}

func (c *Client) HasTokenExpired() bool {
	return c.tokenClient.Token.IsExpired()
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

	nextLink := payload.OdataNextLink

	for nextLink != "" {
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

		var buffer *GroupsListResponse

		err = json.Unmarshal(rawData, &buffer)
		if err != nil {
			return nil, err
		}

		nextLink = buffer.OdataNextLink

		payload.Value = append(payload.Value, buffer.Value...)
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

func (c *Client) CreateAdministrativeUnitGroup(ctx context.Context, requestPayload CreateAdministrativeUnitGroupRequest) (*CreateAdministrativeUnitGroupResponse, error) {
	serialised, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://graph.microsoft.com/v1.0/directory/administrativeUnits/%s/members",
			requestPayload.ParentAdministrativeUnitId), bytes.NewBuffer(serialised))
	if err != nil {
		return nil, err
	}
	c.prepareJsonRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, ApiError{resp.StatusCode}
	}

	var payload CreateAdministrativeUnitGroupResponse
	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
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
	c.prepareJsonRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		if resp.StatusCode == 404 {
			util.Logger.Info(fmt.Sprintf("User %s not found, skipping", upn), zap.String("jobName", "capSvcToAad")) //TODO: Move this outside of azure client
			return nil
		}

		if resp.StatusCode == 403 {
			util.Logger.Info("Response returned with unexpected 403. Skipping entry", zap.String("jobName", "capSvcToAad")) //TODO: Move this outside of azure client
			return nil
		}

		return errors.New(fmt.Sprintf("%d", resp.StatusCode))
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
	c.prepareJsonRequest(req)

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

	payload.tokenClient = util.NewTokenClient(payload.getNewToken)

	return payload
}

const AZURE_CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"
const AZURE_CAPABILITY_GROUP_MAIL_PREFIX = "ci-ssu_cap_"

func GenerateAzureGroupDisplayName(name string) string {
	return fmt.Sprintf("%s %s", AZURE_CAPABILITY_GROUP_PREFIX, name)
}

func GenerateAzureGroupMailPrefix(name string) string {
	return fmt.Sprintf("%s%s", AZURE_CAPABILITY_GROUP_MAIL_PREFIX, name)
}
