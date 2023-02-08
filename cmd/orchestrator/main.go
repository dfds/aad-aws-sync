package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	_ "go.dfds.cloud/aad-aws-sync/cmd/orchestrator/docs"
	"go.dfds.cloud/aad-aws-sync/internal/orchestrator"

	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func metricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// PostAzure2Aws             godoc
// @Summary      Trigger a run of the Azure2AWS Job
// @Description  Triggers a run of the Azure2AWS Job and returns success
// @Tags         azure2aws
// @Produce      json
// @Success      204
// @Router       /azure2aws [post]
func runAzure2Aws(c *gin.Context) {
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not yet configured"})
}

// PostAws2K8s             godoc
// @Summary      Trigger a run of the AWS2K8s Job
// @Description  Triggers a run of the AWS2K8s Job and returns success
// @Tags         aws2k8s
// @Produce      json
// @Success      204
// @Router       /aws2k8s [post]
func runAws2K8s(c *gin.Context) {
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not yet configured"})
}

// AwsMapping             godoc
// @Summary      Trigger a run of the AwsMapping Job
// @Description  Triggers a run of the AwsMapping Job and returns success
// @Tags         awsmapping
// @Produce      json
// @Success      204
// @Router       /awsmapping [post]
func runAwsMapping(c *gin.Context) {
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not yet configured"})
}

// CapSvc2Azure            godoc
// @Summary      Trigger a run of the CapSvc2Azure Job
// @Description  Triggers a run of the CapSvc2Azure Job and returns success
// @Tags         capsvc2azure
// @Produce      json
// @Success      204
// @Router       /capsvc2azure [post]
func runCapSvc2Azure(c *gin.Context) {
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not yet configured"})
}

// main
// Sets up:
// - Prometheus metrics
// - Graceful shutdown via Context
// - Background jobs via Orchestrator
// - HTTP server
// @title           AAD AWS Sync
func main() {
	util.InitializeLogger()
	defer util.Logger.Sync()

	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Unable to load app config", err)
	}

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

	go func() {
		app := fiber.New(fiber.Config{DisableStartupMessage: true})
		app.Use(pprof.New())

		app.Listen(":8081")
	}()

	router := gin.Default()
	router.GET("/metrics", metricsHandler())
	router.POST("/azure2aws", runAzure2Aws)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.Run("localhost:8080")

	backgroundJobWg.Wait()
	util.Logger.Info("All background jobs stopped")
}
