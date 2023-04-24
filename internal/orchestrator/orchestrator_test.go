package orchestrator

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"sync"
	"testing"
)

func TestJob_Run(t *testing.T) {
	util.InitializeLogger()
	j := &Job{
		Name:    "dummy",
		Status:  &SyncStatus{active: false},
		context: context.Background(),
		handler: func(ctx context.Context) error {
			return nil
		},
		wg:              &sync.WaitGroup{},
		ScheduleEnabled: true,
	}

	j.Run()
	j.wg.Wait()

	j.Status.SetStatus(true)
	j.handler = func(ctx context.Context) error {
		return errors.New("dummy")
	}

	j.Run()
	j.wg.Wait()
}

func TestNewOrchestrator(t *testing.T) {
	orc := NewOrchestrator(context.Background(), &sync.WaitGroup{})
	assert.NotNil(t, orc)
}

func TestOrchestrator_AadToAwsSyncStatusProgress(t *testing.T) {
	orc := NewOrchestrator(context.Background(), &sync.WaitGroup{})
	assert.False(t, orc.AadToAwsSyncStatusProgress())
}

func TestOrchestrator_AwsToK8sSyncStatusProgress(t *testing.T) {
	orc := NewOrchestrator(context.Background(), &sync.WaitGroup{})
	assert.False(t, orc.AwsToK8sSyncStatusProgress())
}

func TestOrchestrator_CapSvcToAadSyncProgress(t *testing.T) {
	orc := NewOrchestrator(context.Background(), &sync.WaitGroup{})
	assert.False(t, orc.CapSvcToAadSyncProgress())
}

func TestOrchestrator_Init(t *testing.T) {
	orc := NewOrchestrator(context.Background(), &sync.WaitGroup{})
	orc.Init(config.Config{})
}

func TestSyncStatus_InProgress(t *testing.T) {
	ss := &SyncStatus{}
	assert.False(t, ss.InProgress())
}

func TestSyncStatus_SetStatus(t *testing.T) {
	ss := &SyncStatus{}
	assert.False(t, ss.InProgress())
	ss.SetStatus(true)
	assert.True(t, ss.InProgress())
}
