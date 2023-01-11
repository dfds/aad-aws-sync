package main

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	http.Handle("/metrics", promhttp.Handler())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backgroundJobWg := &sync.WaitGroup{}
	go func() {
		util.Logger.Info("Initialising Orchestrator")
		orc := util.NewOrchestrator(ctx, backgroundJobWg)
		orc.Init()
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
