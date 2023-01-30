package main

import (
	"context"
	"go.dfds.cloud/aad-aws-sync/internal/orchestrator"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

// main
// Sets up:
// - Prometheus metrics
// - Graceful shutdown via Context
// - Background jobs via Orchestrator
// - HTTP server
func main() {
	util.InitializeLogger()
	defer util.Logger.Sync()

	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Unable to load app config", err)
	}

	http.Handle("/metrics", promhttp.Handler())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := gocron.NewScheduler(time.UTC)
	s.SingletonModeAll()

	backgroundJobWg := &sync.WaitGroup{}
	go func() {
		util.Logger.Info("Initialising Orchestrator")
		orc := orchestrator.NewOrchestrator(ctx, backgroundJobWg)
		orc.Init(conf)
		for _, job := range orc.Jobs {
			_, err := s.Every(conf.Scheduler.Frequency).DoWithJobDetails(func(j *orchestrator.Job, c gocron.Job) {
				j.Run()
			}, job)

			if err != nil {
				log.Fatal(err)
			}
		}

		s.StartBlocking()
	}()

	httpServer := &http.Server{Addr: ":8080"}
	go func() {
		<-ctx.Done()
		util.Logger.Info("Shutting down HTTP server")
		if err := httpServer.Shutdown(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}

	backgroundJobWg.Wait()
	util.Logger.Info("All background jobs stopped")
}
