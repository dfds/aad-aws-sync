package handler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/joomcode/errorx"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

const TIUserGroupName = "tiUserGroup"
const tiUserGroupTraversalConcurrency = 10

func TIUserGroupHandler(ctx context.Context) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	jobConf := conf.Handler.TIUserGroup
	if jobConf.RootUserID == "" {
		return fmt.Errorf("%s: AAS_HANDLER_TIUSERGROUP_ROOTUSERID is required but empty", TIUserGroupName)
	}

	azureClient := azure.NewAzureClient(azure.Config{
		TenantId:             conf.Azure.TenantId,
		ClientId:             conf.Azure.ClientId,
		ClientSecret:         conf.Azure.ClientSecret,
		InternalDomainSuffix: conf.Azure.InternalDomainSuffix,
		RateLimitPerSec:      conf.Azure.RateLimitPerSec,
		RateLimitBurst:       conf.Azure.RateLimitBurst,
		MaxRetries:           conf.Azure.MaxRetries,
	})

	rootUser, err := azureClient.GetUserViaUPN(jobConf.RootUserID)
	if err != nil {
		return fmt.Errorf("%s: resolving root user %s: %w", TIUserGroupName, jobConf.RootUserID, err)
	}
	if rootUser == nil || rootUser.ID == "" {
		return fmt.Errorf("%s: root user %s not found", TIUserGroupName, jobConf.RootUserID)
	}
	util.Logger.Debug(fmt.Sprintf("Resolved root user %s (%s)", rootUser.UserPrincipalName, rootUser.ID),
		zap.String("jobName", TIUserGroupName))

	// 2. Build the desired membership set by walking directReports from the root.
	util.Logger.Debug(fmt.Sprintf("Walking directReports hierarchy from root (concurrency %d)", tiUserGroupTraversalConcurrency),
		zap.String("jobName", TIUserGroupName))
	desired, err := collectReports(ctx, azureClient, rootUser.ID, tiUserGroupTraversalConcurrency)
	if err != nil {
		return fmt.Errorf("%s: building desired membership: %w", TIUserGroupName, err)
	}

	if _, ok := desired[rootUser.ID]; !ok {
		desired[rootUser.ID] = &azure.Member{
			ID:                rootUser.ID,
			DisplayName:       rootUser.DisplayName,
			UserPrincipalName: rootUser.UserPrincipalName,
			Mail:              rootUser.Mail,
		}
	}
	util.Logger.Info(fmt.Sprintf("Desired membership resolved: %d users", len(desired)), zap.String("jobName", TIUserGroupName))

	aUnits, err := azureClient.GetAdministrativeUnits()
	if err != nil {
		return fmt.Errorf("%s: listing administrative units: %w", TIUserGroupName, err)
	}
	aUnit := aUnits.GetUnit(jobConf.AdministrativeUnitName)
	if aUnit == nil {
		return fmt.Errorf("%s: administrative unit %q not found", TIUserGroupName, jobConf.AdministrativeUnitName)
	}

	aUnitMembers, err := azureClient.GetAdministrativeUnitMembers(aUnit.ID)
	if err != nil {
		return fmt.Errorf("%s: listing administrative unit members: %w", TIUserGroupName, err)
	}

	var groupID string
	groupCreated := false
	for _, m := range aUnitMembers.Value {
		if m.DisplayName == jobConf.GroupDisplayName {
			groupID = m.ID
			break
		}
	}

	if groupID == "" {
		util.Logger.Info(fmt.Sprintf("Group %q not found in AU %q, creating.", jobConf.GroupDisplayName, jobConf.AdministrativeUnitName), zap.String("jobName", TIUserGroupName))
		created, err := azureClient.CreateAdministrativeUnitGroup(ctx, azure.CreateAdministrativeUnitGroupRequest{
			OdataType:                  "#Microsoft.Graph.Group",
			Description:                "[Automated] - aad-aws-sync",
			DisplayName:                jobConf.GroupDisplayName,
			MailNickname:               jobConf.MailNickname,
			GroupTypes:                 []interface{}{},
			MailEnabled:                false,
			SecurityEnabled:            true,
			ParentAdministrativeUnitId: aUnit.ID,
		})
		if err != nil {
			return fmt.Errorf("%s: creating group %q: %w", TIUserGroupName, jobConf.GroupDisplayName, err)
		}
		groupID = created.ID
		groupCreated = true
	}
	util.Logger.Debug(fmt.Sprintf("Using group %q (%s) in AU %q", jobConf.GroupDisplayName, groupID, jobConf.AdministrativeUnitName),
		zap.String("jobName", TIUserGroupName))

	// 4. Load the group's current members. A group we just created may not be
	//    queryable yet due to Graph replication lag, so tolerate transient 404s
	//    there before giving up.
	var actualResp *azure.GroupMembers
	if groupCreated {
		actualResp, err = waitForNewGroupMembers(ctx, azureClient, groupID)
	} else {
		actualResp, err = azureClient.GetGroupMembers(groupID)
	}
	if err != nil {
		return fmt.Errorf("%s: listing group members: %w", TIUserGroupName, err)
	}

	type actualMember struct {
		id  string
		upn string
	}
	actual := make(map[string]actualMember, len(actualResp.Value))
	for _, m := range actualResp.Value {
		actual[m.ID] = actualMember{id: m.ID, upn: m.UserPrincipalName}
	}
	util.Logger.Debug(fmt.Sprintf("Group currently has %d members", len(actual)),
		zap.String("jobName", TIUserGroupName))

	// 5. Compute the reconcile.
	var toAdd []*azure.Member
	for id, member := range desired {
		if _, ok := actual[id]; !ok {
			toAdd = append(toAdd, member)
		}
	}

	var toRemove []actualMember
	for id, m := range actual {
		if _, ok := desired[id]; ok {
			continue
		}
		// Ignored service accounts are left untouched, never removed.
		if m.upn != "" && ShouldIgnoreUser(m.upn) {
			continue
		}
		toRemove = append(toRemove, m)
	}

	// Safety floor: refuse to proceed if a single run would remove more than the
	// configured fraction of the current membership — a strong signal of a
	// misconfigured root or a bad enumeration. Skipped on an empty group (first
	// run) where no removals are possible.
	if len(actual) > 0 {
		fraction := float64(len(toRemove)) / float64(len(actual))
		if fraction > jobConf.MaxRemovalFraction {
			util.Logger.Error(fmt.Sprintf("Safety floor tripped: run would remove %d of %d members (%.1f%% > %.1f%% max); aborting without changes.",
				len(toRemove), len(actual), fraction*100, jobConf.MaxRemovalFraction*100),
				zap.String("jobName", TIUserGroupName))
			return fmt.Errorf("%s: removal fraction %.2f exceeds max %.2f, aborting", TIUserGroupName, fraction, jobConf.MaxRemovalFraction)
		}
	}

	// 6. Apply additions then removals.
	util.Logger.Info(fmt.Sprintf("Reconcile plan: %d to add, %d to remove", len(toAdd), len(toRemove)),
		zap.String("jobName", TIUserGroupName))

	added := 0
	for _, member := range toAdd {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", TIUserGroupName))
			return nil
		default:
		}

		util.Logger.Info(fmt.Sprintf("Adding member %s (%s)", member.UserPrincipalName, member.ID),
			zap.String("jobName", TIUserGroupName))
		err := azureClient.AddGroupMember(groupID, member.ID)
		if err != nil {
			if errorx.IsOfType(err, azure.AdUserNotFound) || errorx.IsOfType(err, azure.HttpError403) {
				util.Logger.Error(err.Error(), zap.String("jobName", TIUserGroupName))
				continue
			}
			return fmt.Errorf("%s: adding member %s to group %s: %w", TIUserGroupName, member.ID, jobConf.GroupDisplayName, err)
		}
		added++
	}

	removed := 0
	for _, m := range toRemove {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", TIUserGroupName))
			return nil
		default:
		}

		util.Logger.Info(fmt.Sprintf("Removing member %s (%s)", m.upn, m.id),
			zap.String("jobName", TIUserGroupName))
		err := azureClient.DeleteGroupMember(groupID, m.id)
		if err != nil {
			return fmt.Errorf("%s: removing member %s from group %s: %w", TIUserGroupName, m.id, jobConf.GroupDisplayName, err)
		}
		removed++
	}

	util.Logger.Info(fmt.Sprintf("Reconcile complete for %q: %d added, %d removed (%d desired, %d previously present)",
		jobConf.GroupDisplayName, added, removed, len(desired), len(actual)), zap.String("jobName", TIUserGroupName))

	return nil
}

const (
	tiUserGroupReadyMaxAttempts = 20
	tiUserGroupReadyInterval    = 3 * time.Second
)

func waitForNewGroupMembers(ctx context.Context, client *azure.Client, groupID string) (*azure.GroupMembers, error) {
	for attempt := 1; ; attempt++ {
		resp, err := client.GetGroupMembers(groupID)
		if err == nil {
			return resp, nil
		}
		if !errorx.IsOfType(err, azure.HttpError404) || attempt >= tiUserGroupReadyMaxAttempts {
			return nil, err
		}

		util.Logger.Debug(fmt.Sprintf("New group %s not yet replicated (attempt %d/%d), retrying in %s",
			groupID, attempt, tiUserGroupReadyMaxAttempts, tiUserGroupReadyInterval), zap.String("jobName", TIUserGroupName))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(tiUserGroupReadyInterval):
		}
	}
}

func tiUserGroupIncludeUser(r *azure.DirectReport) bool {
	if r.OdataType != "" && !strings.EqualFold(r.OdataType, "#microsoft.graph.user") {
		return false
	}
	if r.AccountEnabled != nil && !*r.AccountEnabled {
		return false
	}
	if ShouldIgnoreUser(r.UserPrincipalName) {
		return false
	}
	return true
}

// tiUserGroupWalkProgressEvery controls how often the walk emits a debug progress heartbeat
const tiUserGroupWalkProgressEvery = 25

// reportCollector shared state
type reportCollector struct {
	ctx       context.Context
	cancel    context.CancelFunc
	client    *azure.Client
	sem       *semaphore.Weighted
	wg        sync.WaitGroup
	mu        sync.Mutex
	result    map[string]*azure.Member
	visited   map[string]bool
	processed int
	firstErr  error
}

// collectReports fetches users managed by rootid, and then recursively managers and users all the way until no managers are found
func collectReports(ctx context.Context, client *azure.Client, rootID string, concurrency int64) (map[string]*azure.Member, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c := &reportCollector{
		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		sem:     semaphore.NewWeighted(concurrency),
		result:  make(map[string]*azure.Member),
		visited: map[string]bool{rootID: true},
	}

	c.wg.Add(1)
	go c.visit(rootID)
	c.wg.Wait()

	if c.firstErr != nil {
		return nil, c.firstErr
	}
	return c.result, nil
}

// setErr records the first error and cancels the walk; later errors are dropped.
func (c *reportCollector) setErr(err error) {
	c.mu.Lock()
	if c.firstErr == nil {
		c.firstErr = err
		c.cancel()
	}
	c.mu.Unlock()
}

// markVisited records id and reports whether it was newly seen (i.e. should be traversed).
func (c *reportCollector) markVisited(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.visited[id] {
		return false
	}
	c.visited[id] = true
	return true
}

func (c *reportCollector) visit(id string) {
	defer c.wg.Done()

	if err := c.sem.Acquire(c.ctx, 1); err != nil {
		return
	}
	defer c.sem.Release(1)

	resp, err := c.client.GetUserDirectReports(c.ctx, id)
	if err != nil {
		c.setErr(fmt.Errorf("fetching directReports for %s: %w", id, err))
		return
	}

	c.mu.Lock()
	c.processed++
	processed := c.processed
	c.mu.Unlock()
	util.Logger.Debug(fmt.Sprintf("Walked directReports for %s: %d reports", id, len(resp.Value)),
		zap.String("jobName", TIUserGroupName))
	if processed%tiUserGroupWalkProgressEvery == 0 {
		c.mu.Lock()
		included := len(c.result)
		c.mu.Unlock()
		util.Logger.Debug(fmt.Sprintf("Walk progress: %d nodes fetched, %d users included so far", processed, included),
			zap.String("jobName", TIUserGroupName))
	}

	for _, r := range resp.Value {
		if r.ID == "" {
			continue
		}
		if tiUserGroupIncludeUser(r) {
			c.mu.Lock()
			if _, ok := c.result[r.ID]; !ok {
				c.result[r.ID] = &azure.Member{
					ID:                r.ID,
					DisplayName:       r.DisplayName,
					UserPrincipalName: r.UserPrincipalName,
					Mail:              r.Mail,
				}
			}
			c.mu.Unlock()
		}
		if c.markVisited(r.ID) {
			c.wg.Add(1)
			go c.visit(r.ID)
		}
	}
}
