package util

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

const CapabilityServiceToAzureAdName = "capSvcToAad"
const AzureAdToAwsName = "aadToAws"
const AwsToKubernetesName = "awsToK8s"
const AwsMappingName = "awsMapping"

var currentJobsGauge prometheus.Gauge = promauto.NewGauge(prometheus.GaugeOpts{
	Name:      "jobs_running",
	Help:      "Current jobs that are running",
	Namespace: "aad_aws_sync",
})

var currentJobStatus *prometheus.GaugeVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name:      "job_is_running",
	Help:      "Is {job_name} running. 1 = in progress, 0 = not running",
	Namespace: "aad_aws_sync",
}, []string{"name"})

var jobFailedCount *prometheus.GaugeVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name:      "job_failed_count",
	Help:      "How many times has {job_name} failed.",
	Namespace: "aad_aws_sync",
}, []string{"name"})

var jobSuccessfulCount *prometheus.GaugeVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name:      "job_success_count",
	Help:      "How many times has {job_name} successfully completed.",
	Namespace: "aad_aws_sync",
}, []string{"name"})

// Orchestrator
// Used for managing long-lived fully fledged sync jobs
type Orchestrator struct {
	capSvcToAadSyncStatus *SyncStatus
	aadToAwsSyncStatus    *SyncStatus
	awsMappingStatus      *SyncStatus
	awsToK8sSyncStatus    *SyncStatus
	jobs                  map[string]*Job
	ctx                   context.Context
	wg                    *sync.WaitGroup
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

func NewOrchestrator(ctx context.Context, wg *sync.WaitGroup) *Orchestrator {
	return &Orchestrator{
		capSvcToAadSyncStatus: &SyncStatus{active: false},
		aadToAwsSyncStatus:    &SyncStatus{active: false},
		awsMappingStatus:      &SyncStatus{active: false},
		awsToK8sSyncStatus:    &SyncStatus{active: false},
		jobs:                  map[string]*Job{},
		ctx:                   ctx,
		wg:                    wg,
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
		wg:      o.wg,
		handler: func() error {
			fmt.Println("Doing capsvc 2 azure work")
			fakeJobGen(o.ctx, "Adding Capability", CapabilityServiceToAzureAdName)
			return nil
		},
	}

	o.jobs[AzureAdToAwsName] = &Job{
		name:    AzureAdToAwsName,
		status:  o.aadToAwsSyncStatus,
		context: context.TODO(),
		wg:      o.wg,
		handler: func() error {
			fmt.Println("Doing azure 2 aws work")
			fakeJobGen(o.ctx, "Adding AD group to app registration for SCIM", AzureAdToAwsName)
			return nil
		},
	}

	o.jobs[AwsMappingName] = &Job{
		name:    AwsMappingName,
		status:  o.awsMappingStatus,
		context: context.TODO(),
		wg:      o.wg,
		handler: func() error {
			fmt.Println("Doing aws mapping work")
			fakeJobGen(o.ctx, "Mapping AWS group to AWS account", AwsMappingName)
			return nil
		},
	}

	o.jobs[AwsToKubernetesName] = &Job{
		name:    AwsToKubernetesName,
		status:  o.awsToK8sSyncStatus,
		context: context.TODO(),
		wg:      o.wg,
		handler: func() error {
			fmt.Println("Doing aws 2 k8s work")
			fakeJobGen(o.ctx, "Mapping AWS SSO role to namespace", AwsToKubernetesName)
			return nil
		},
	}

	for jobName, job := range o.jobs {
		Logger.Info(fmt.Sprintf("Starting %s job", jobName), zap.String("jobName", jobName))
		job.Run()
		//job.Run() // Run twice to check if mutex is working as intended
	}
}

type Job struct {
	name    string
	status  *SyncStatus
	context context.Context
	handler func() error
	wg      *sync.WaitGroup
}

func (j *Job) Run() {
	if j.status.InProgress() {
		Logger.Error("Can't start Job because Job is already in progress.", zap.String("jobName", j.name))
		return
	}
	j.status.SetStatus(true)
	j.wg.Add(1)
	currentJobsGauge.Inc()
	currentJobStatus.WithLabelValues(j.name).Set(1)

	go func() {
		defer j.wg.Done()
		err := j.handler()
		if err != nil {
			jobFailedCount.WithLabelValues(j.name).Inc()
		} else {
			jobSuccessfulCount.WithLabelValues(j.name).Inc()
		}
		currentJobsGauge.Dec()
		currentJobStatus.WithLabelValues(j.name).Set(0)
	}()
}

func fakeJobGen(ctx context.Context, msg string, jobName string) {
	entityCount := rand.Intn(100-20) + 20
	sleepInSecs := rand.Intn(7-2) + 2

	for i := 0; i < entityCount; i++ {
		select {
		case <-ctx.Done():
			Logger.Info("Job cancelled", zap.String("jobName", jobName))
			return
		default:
			Logger.Info(fmt.Sprintf("%s %d", msg, i), zap.String("jobName", jobName))
			time.Sleep(time.Second * time.Duration(sleepInSecs))
		}
	}
}
