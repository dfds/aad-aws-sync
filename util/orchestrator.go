package util

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"sync"
)

const CapabilityServiceToAzureAdName = "capSvcToAad"
const AzureAdToAwsName = "aadToAws"
const AwsToKubernetesName = "awsToK8s"

var currentJobsGauge prometheus.Gauge = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "jobs_running",
	Help: "Current jobs that are running",
})

// Orchestrator
// Used for managing long-lived fully fledged sync jobs
type Orchestrator struct {
	capSvcToAadSyncStatus *SyncStatus
	aadToAwsSyncStatus    *SyncStatus
	awsToK8sSyncStatus    *SyncStatus
	jobs                  map[string]*Job
}

type SyncStatus struct {
	mu     sync.Mutex
	active bool
}

func (s *SyncStatus) InProgress() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *SyncStatus) SetStatus(status bool) {
	s.mu.Lock()
	s.active = status
	s.mu.Unlock()
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		capSvcToAadSyncStatus: &SyncStatus{active: false},
		aadToAwsSyncStatus:    &SyncStatus{active: false},
		awsToK8sSyncStatus:    &SyncStatus{active: false},
		jobs:                  map[string]*Job{},
	}
}

func (o *Orchestrator) AadToAwsSyncStatusProgress() bool {
	return o.aadToAwsSyncStatus.InProgress()
}

func (o *Orchestrator) AwsToK8sSyncStatusProgress() bool {
	return o.awsToK8sSyncStatus.InProgress()
}

func (o *Orchestrator) CapSvcToAadSyncProgress() bool {
	return o.capSvcToAadSyncStatus.InProgress()
}

func (o *Orchestrator) Init() {
	o.jobs[CapabilityServiceToAzureAdName] = &Job{
		name:    CapabilityServiceToAzureAdName,
		status:  o.capSvcToAadSyncStatus,
		context: context.TODO(),
		handler: func() {
			go func() {
				currentJobsGauge.Dec()
			}()
		},
	}

	o.jobs[AzureAdToAwsName] = &Job{
		name:    AzureAdToAwsName,
		status:  o.aadToAwsSyncStatus,
		context: context.TODO(),
		handler: func() {

		},
	}

	o.jobs[AwsToKubernetesName] = &Job{
		name:    AwsToKubernetesName,
		status:  o.awsToK8sSyncStatus,
		context: context.TODO(),
		handler: func() {

		},
	}

	for jobName, job := range o.jobs {
		Logger.Info(fmt.Sprintf("Starting %s job", jobName), zap.String("jobName", jobName))
		job.Run()
		job.Run() // Run twice to check if mutex is working as intended
	}
}

type Job struct {
	name    string
	status  *SyncStatus
	context context.Context
	handler func()
}

func (j *Job) Run() {
	if j.status.InProgress() {
		Logger.Error("Can't start Job because Job is already in progress.", zap.String("jobName", j.name))
		return
	}
	currentJobsGauge.Inc()
	j.status.SetStatus(true)
	j.handler() // TODO: Put in a separate goroutine
}
