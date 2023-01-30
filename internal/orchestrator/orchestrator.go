package orchestrator

import (
	"context"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"math/rand"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
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
	Jobs                  map[string]*Job
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
		Jobs:                  map[string]*Job{},
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
	o.Jobs[CapabilityServiceToAzureAdName] = &Job{
		name:    CapabilityServiceToAzureAdName,
		status:  o.capSvcToAadSyncStatus,
		context: o.ctx,
		wg:      o.wg,
		handler: handler.Capsvc2AadHandler,
	}

	o.Jobs[AzureAdToAwsName] = &Job{
		name:    AzureAdToAwsName,
		status:  o.aadToAwsSyncStatus,
		context: o.ctx,
		wg:      o.wg,
		handler: handler.Azure2AwsHandler,
	}

	o.Jobs[AwsMappingName] = &Job{
		name:    AwsMappingName,
		status:  o.awsMappingStatus,
		context: o.ctx,
		wg:      o.wg,
		handler: handler.AwsMappingHandler,
	}

	o.Jobs[AwsToKubernetesName] = &Job{
		name:    AwsToKubernetesName,
		status:  o.awsToK8sSyncStatus,
		context: o.ctx,
		wg:      o.wg,
		handler: handler.Aws2K8sHandler,
	}

	// Commented out by command of Emil I
	// for jobName, job := range o.Jobs {
	// 	Logger.Info(fmt.Sprintf("Starting %s job", jobName), zap.String("jobName", jobName))
	// 	job.Run()
	// }
}

type Job struct {
	name    string
	status  *SyncStatus
	context context.Context
	handler func(ctx context.Context) error
	wg      *sync.WaitGroup
}

func (j *Job) Run() {
	if j.status.InProgress() {
		util.Logger.Warn("Can't start Job because Job is already in progress.", zap.String("jobName", j.name))
		return
	}
	j.status.SetStatus(true)
	j.wg.Add(1)
	currentJobsGauge.Inc()
	currentJobStatus.WithLabelValues(j.name).Set(1)

	go func() {
		defer j.wg.Done()
		err := j.handler(j.context)
		if err != nil {
			jobFailedCount.WithLabelValues(j.name).Inc()
		} else {
			jobSuccessfulCount.WithLabelValues(j.name).Inc()
		}
		currentJobsGauge.Dec()
		currentJobStatus.WithLabelValues(j.name).Set(0)
		j.status.SetStatus(false)
	}()
}

func fakeJobGen(ctx context.Context, msg string, jobName string) {
	entityCount := rand.Intn(100-20) + 20
	sleepInSecs := rand.Intn(7-2) + 2

	for i := 0; i < entityCount; i++ {
		select {
		case <-ctx.Done():
			util.Logger.Info("Job cancelled", zap.String("jobName", jobName))
			return
		default:
			util.Logger.Info(fmt.Sprintf("%s %d", msg, i), zap.String("jobName", jobName))
			time.Sleep(time.Second * time.Duration(sleepInSecs))
		}
	}
}