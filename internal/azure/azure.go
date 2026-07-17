package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"

	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"k8s.io/utils/env"
)

// TODO look into using: https://github.com/microsoftgraph/msgraph-sdk-go

type Client struct {
	httpClient  *http.Client
	tokenClient *util.TokenClient
	config      Config
	limiter     *rate.Limiter
	maxRetries  int
}

type Config struct {
	TenantId             string `json:"tenantId"`
	ClientId             string `json:"clientId"`
	ClientSecret         string `json:"clientSecret"`
	InternalDomainSuffix string `json:"internalDomainSuffix"`
	RateLimitPerSec      int    `json:"rateLimitPerSec"`
	RateLimitBurst       int    `json:"rateLimitBurst"`
	MaxRetries           int    `json:"maxRetries"`
}

const (
	// Microsoft Graph allows 455 requests / 10s (~45/s) per app per tenant for
	// the directory resources this service uses (users, groups, administrative
	// units). 35/s leaves headroom below that while the shared limiter keeps all
	// concurrent jobs within a single budget
	defaultRateLimitPerSec = 35
	defaultRateLimitBurst  = 35
	defaultMaxRetries      = 5
	maxRetryBackoff        = 30 * time.Second
)

var (
	sharedLimiterOnce sync.Once
	sharedLimiter     *rate.Limiter
)

func sharedRateLimiter(ratePerSec, burst int) *rate.Limiter {
	sharedLimiterOnce.Do(func() {
		sharedLimiter = rate.NewLimiter(rate.Limit(ratePerSec), burst)
	})
	return sharedLimiter
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Wait(req.Context()); err != nil {
				return nil, err
			}
		}

		if attempt > 0 && req.Body != nil && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
			return resp, nil
		}

		if attempt >= c.maxRetries {
			return resp, nil
		}

		delay := retryAfterDelay(resp, attempt)
		resp.Body.Close()

		util.Logger.Debug(fmt.Sprintf("Graph throttled request (status %d), retrying in %s (attempt %d/%d)", resp.StatusCode, delay, attempt+1, c.maxRetries))

		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}

func retryAfterDelay(resp *http.Response, attempt int) time.Duration {
	if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}

	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if backoff > maxRetryBackoff {
		backoff = maxRetryBackoff
	}
	return backoff
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
	reqPayload.Set("scope", "https://graph.microsoft.com/.default")
	reqPayload.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequest("POST", fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.config.TenantId), strings.NewReader(reqPayload.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.do(req)
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

func (c *Client) HasTokenExpired() bool {
	return c.tokenClient.Token.IsExpired()
}

func (c *Client) GetGroups(prefix string) (*GroupsListResponse, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/groups", nil)
	if err != nil {
		return nil, err
	}
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$filter", fmt.Sprintf("startswith(displayName,'%s')", prefix))
	req.URL.RawQuery = urlQueryValues.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

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
		err = c.prepareHttpRequest(req)
		if err != nil {
			return nil, err
		}

		resp, err := c.do(req)
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
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$filter", "startswith(displayName,'Team - Cloud Engineering')")
	req.URL.RawQuery = urlQueryValues.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

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
	err = c.prepareJsonRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, unexpectedStatusError(fmt.Sprintf("CreateAdministrativeUnitGroup(group %s, parentAU %s)", requestPayload.DisplayName, requestPayload.ParentAdministrativeUnitId), resp.StatusCode, rawData)
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
	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return unexpectedStatusError(fmt.Sprintf("DeleteAdministrativeUnitGroup(aUnit %s, group %s)", aUnitId, groupId), resp.StatusCode, body)
	}

	return nil
}

func (c *Client) PopulateGroupsWithMembers(groups *GroupsListResponse) (map[string]*Group, error) {
	var payload map[string]*Group
	ctx := context.Background()
	var waitGroup sync.WaitGroup
	sem := semaphore.NewWeighted(50)
	var lock *sync.Mutex = &sync.Mutex{}

	for _, grp := range groups.Value {
		waitGroup.Add(1)

		grp := grp
		go func() {
			sem.Acquire(ctx, 1)
			defer sem.Release(1)
			defer waitGroup.Done()

			group := &Group{
				DisplayName: grp.DisplayName,
				Members:     []*Member{},
				ID:          grp.ID,
			}
			groupMembers, err := c.GetGroupMembers(grp.ID)
			if err != nil {
				fmt.Println(fmt.Sprintf("GetGroupMembers failed: %v", err), zap.Error(err))
			}

			for _, groupMember := range groupMembers.Value {
				group.Members = append(group.Members, &Member{
					ID:                groupMember.ID,
					DisplayName:       groupMember.DisplayName,
					UserPrincipalName: groupMember.UserPrincipalName,
				})
			}

			lock.Lock()
			payload[group.DisplayName] = group
			lock.Unlock()
		}()
	}

	waitGroup.Wait()

	return payload, nil
}

func (c *Client) AddGroupMember(groupId string, upn string) error {
	requestPayload := AddGroupMemberRequest{
		OdataId: fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s", url.QueryEscape(upn)),
	}

	serialised, err := json.Marshal(requestPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/members/$ref", groupId), bytes.NewBuffer([]byte(serialised)))
	if err != nil {
		return err
	}
	err = c.prepareJsonRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		if resp.StatusCode == 404 {
			return AdUserNotFound.New(fmt.Sprintf("User %s not found, skipping", upn))
		}

		if resp.StatusCode == 403 {
			return HttpError403.New("Response returned with unexpected 403. Skipping entry")
		}

		if resp.StatusCode == 400 {
			util.Logger.Info("Response returned with unexpected 400. User might already be a member.")
			return nil
		}

		return HttpError.New(fmt.Sprintf("Unexpected HTTP response. Status code: %d", resp.StatusCode))
	}

	return nil
}

func (c *Client) DeleteGroupMember(groupId string, memberId string) error {

	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/members/%s/$ref", groupId, memberId), nil)
	if err != nil {
		return err
	}
	err = c.prepareJsonRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		if resp.StatusCode == 404 {
			util.Logger.Info(fmt.Sprintf("User %s not found, skipping", memberId), zap.String("jobName", "capSvcToAad")) //TODO: Move this outside of azure client
			return nil
		}

		if resp.StatusCode == 403 {
			util.Logger.Info("Response returned with unexpected 403. Skipping entry", zap.String("jobName", "capSvcToAad")) //TODO: Move this outside of azure client
			return nil
		}

		return HttpError.New(fmt.Sprintf("Unexpected HTTP response. Status code: %d", resp.StatusCode))
	}

	return nil
}

func (c *Client) GetAdministrativeUnitMembers(id string) (*GetAdministrativeUnitMembersResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/directory/administrativeUnits/%s/members", id), nil)
	if err != nil {
		return nil, err
	}

	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

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
		err = c.prepareHttpRequest(req)
		if err != nil {
			return nil, err
		}

		resp, err := c.do(req)
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

func (c *Client) GetUserViaUPN(upn string) (*GetUserViaUPNResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s", url.QueryEscape(upn)), nil)
	if err != nil {
		return nil, err
	}
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GetUserViaUPNResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) GetUserViaEmail(email string) (*GetUserViaUPNResponse, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/users", nil)
	if err != nil {
		return nil, err
	}
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	urlQueries := req.URL.Query()
	urlQueries.Add("$top", "5")
	urlQueries.Add("$filter", fmt.Sprintf("mail eq '%s'", email))

	req.URL.RawQuery = urlQueries.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GetUsersResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	if len(payload.Value) == 0 {
		return nil, fmt.Errorf("GetUserViaEmail: no user found for email %s", email)
	}

	return payload.Value[0], nil
}

func (c *Client) IsUserExternal(value string) bool {
	return !strings.HasSuffix(strings.ToLower(value), c.config.InternalDomainSuffix)
}

func (c *Client) GetGroupMembers(id string) (*GroupMembers, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/members", id), nil)
	if err != nil {
		return nil, err
	}
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		// A newly created group can briefly 404 here due to Graph replication lag
		return nil, HttpError404.New(fmt.Sprintf("GetGroupMembers(%s): %s", id, strings.TrimSpace(string(rawData))))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatusError(fmt.Sprintf("GetGroupMembers(%s)", id), resp.StatusCode, rawData)
	}

	var payload *GroupMembers

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
		err = c.prepareHttpRequest(req)
		if err != nil {
			return nil, err
		}

		resp, err := c.do(req)
		if err != nil {
			return nil, err
		}

		rawData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, unexpectedStatusError(fmt.Sprintf("GetGroupMembers(%s)", id), resp.StatusCode, rawData)
		}

		var buffer *GroupMembers

		err = json.Unmarshal(rawData, &buffer)
		if err != nil {
			return nil, err
		}

		nextLink = buffer.OdataNextLink

		payload.Value = append(payload.Value, buffer.Value...)
	}

	return payload, nil
}

func (c *Client) GetUserDirectReports(ctx context.Context, userIdOrUPN string) (*GetDirectReportsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/directReports", url.PathEscape(userIdOrUPN)), nil)
	if err != nil {
		return nil, err
	}
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$select", "id,displayName,userPrincipalName,mail,accountEnabled")
	req.URL.RawQuery = urlQueryValues.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatusError(fmt.Sprintf("GetUserDirectReports(%s)", userIdOrUPN), resp.StatusCode, rawData)
	}

	var payload *GetDirectReportsResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	nextLink := payload.OdataNextLink

	for nextLink != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", nextLink, nil)
		if err != nil {
			return nil, err
		}
		err = c.prepareHttpRequest(req)
		if err != nil {
			return nil, err
		}

		resp, err := c.do(req)
		if err != nil {
			return nil, err
		}

		rawData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, unexpectedStatusError(fmt.Sprintf("GetUserDirectReports(%s)", userIdOrUPN), resp.StatusCode, rawData)
		}

		var buffer *GetDirectReportsResponse

		err = json.Unmarshal(rawData, &buffer)
		if err != nil {
			return nil, err
		}

		nextLink = buffer.OdataNextLink

		payload.Value = append(payload.Value, buffer.Value...)
	}

	return payload, nil
}

func (c *Client) GetApplicationRoles(appId string) (*GetApplicationRolesResponse, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/applications", nil)
	if err != nil {
		return nil, err
	}
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	urlQueryValues := req.URL.Query()
	urlQueryValues.Set("$filter", fmt.Sprintf("appId eq '%s'", appId))
	urlQueryValues.Set("$select", "displayName, appId, appRoles")
	req.URL.RawQuery = urlQueryValues.Encode()

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

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
	err = c.prepareHttpRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

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
		err = c.prepareHttpRequest(req)
		if err != nil {
			return nil, err
		}

		resp, err := c.do(req)
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
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/appRoleAssignments", groupId), bytes.NewBuffer([]byte(serialised)))
	if err != nil {
		return nil, err
	}
	err = c.prepareJsonRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 201 {
		return nil, unexpectedStatusError(fmt.Sprintf("AssignGroupToApplication(app %s, group %s, role %s)", appObjectId, groupId, roleId), resp.StatusCode, rawData)
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
	err = c.prepareHttpRequest(req)
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return unexpectedStatusError(fmt.Sprintf("UnassignGroupFromApplication(group %s, assignment %s)", groupId, assignmentId), resp.StatusCode, body)
	}

	return nil
}

func NewAzureClient(conf Config) *Client {
	ratePerSec := conf.RateLimitPerSec
	if ratePerSec <= 0 {
		ratePerSec = defaultRateLimitPerSec
	}
	burst := conf.RateLimitBurst
	if burst <= 0 {
		burst = defaultRateLimitBurst
	}
	maxRetries := conf.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	payload := &Client{
		httpClient: http.DefaultClient,
		config:     conf,
		limiter:    sharedRateLimiter(ratePerSec, burst),
		maxRetries: maxRetries,
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
