package main

import (
	"context"
	"go.dfds.cloud/aad-aws-sync/internal/event"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.dfds.cloud/aad-aws-sync/internal/middleware"

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
// @Success      201
// @Failure      404
// @Failure      409
// @Failure      500
// @Router       /azure2aws [post]
func runAzure2Aws(c *gin.Context) {
	orc := middleware.GetOrchestrator(c)

	if orc.Jobs["aadToAws"] != nil {
		if !orc.Jobs["aadToAws"].Status.InProgress() {
			orc.Jobs["aadToAws"].Run()
			c.IndentedJSON(http.StatusCreated, gin.H{"message": "job created"})
		} else {
			c.IndentedJSON(http.StatusConflict, gin.H{"message": "job in progress"})
		}
	} else {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not found"})
	}
}

// PostAws2K8s             godoc
// @Summary      Trigger a run of the AWS2K8s Job
// @Description  Triggers a run of the AWS2K8s Job and returns success
// @Tags         aws2k8s
// @Produce      json
// @Success      201
// @Failure      404
// @Failure      409
// @Failure      500
// @Router       /aws2k8s [post]
func runAws2K8s(c *gin.Context) {
	orc := middleware.GetOrchestrator(c)

	if orc.Jobs["awsToK8s"] != nil {
		if !orc.Jobs["awsToK8s"].Status.InProgress() {
			orc.Jobs["awsToK8s"].Run()
			c.IndentedJSON(http.StatusCreated, gin.H{"message": "job created"})
		} else {
			c.IndentedJSON(http.StatusConflict, gin.H{"message": "job in progress"})
		}
	} else {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not found"})
	}
}

// AwsMapping             godoc
// @Summary      Trigger a run of the AwsMapping Job
// @Description  Triggers a run of the AwsMapping Job and returns success
// @Tags         awsmapping
// @Produce      json
// @Success      201
// @Failure      404
// @Failure      409
// @Failure      500
// @Router       /awsmapping [post]
func runAwsMapping(c *gin.Context) {
	orc := middleware.GetOrchestrator(c)

	if orc.Jobs["awsMapping"] != nil {
		if !orc.Jobs["awsMapping"].Status.InProgress() {
			orc.Jobs["awsMapping"].Run()
			c.IndentedJSON(http.StatusCreated, gin.H{"message": "job created"})
		} else {
			c.IndentedJSON(http.StatusConflict, gin.H{"message": "job in progress"})
		}
	} else {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not found"})
	}
}

// CapSvc2Azure            godoc
// @Summary      Trigger a run of the CapSvc2Azure Job
// @Description  Triggers a run of the CapSvc2Azure Job and returns success
// @Tags         capsvc2azure
// @Produce      json
// @Success      201
// @Failure      404
// @Failure      409
// @Failure      500
// @Router       /capsvc2azure [post]
func runCapSvc2Azure(c *gin.Context) {
	orc := middleware.GetOrchestrator(c)

	if orc.Jobs["capSvcToAad"] != nil {
		if !orc.Jobs["capSvcToAad"].Status.InProgress() {
			orc.Jobs["capSvcToAad"].Run()
			c.IndentedJSON(http.StatusCreated, gin.H{"message": "job created"})
		} else {
			c.IndentedJSON(http.StatusConflict, gin.H{"message": "job in progress"})
		}
	} else {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "job not found"})
	}
}

// main
// Sets up:
// - Prometheus metrics
// - Graceful shutdown via Context
// - Background jobs via Orchestrator
// - HTTP server
// @title           AAD AWS Sync
// @version 1
// @basePath /api/v1
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
	orc := orchestrator.NewOrchestrator(ctx, backgroundJobWg)
	orc.Init(conf)
	// Orchestrator goroutine; Handles scheduling jobs
	go func() {
		util.Logger.Info("Initialising Orchestrator")
		for _, job := range orc.Jobs {
			_, err := s.Every(conf.Scheduler.Frequency).DoWithJobDetails(func(j *orchestrator.Job, c gocron.Job) {
				// If conf flag is set to not schedule job, do not do it!
				if j.ScheduleEnabled {
					j.Run()
				}
			}, job)

			if err != nil {
				log.Fatal(err)
			}
		}
		s.StartBlocking()
	}()

	// Event handling
	if conf.EventHandling.Enabled {
		go func() {
			err = event.StartEventHandlers(ctx, conf, backgroundJobWg)
			if err != nil {
				util.Logger.Error("Event loop error, shutting down.", zap.Error(err))
				stop()
			}
		}()
	}

	// Profiling endpoint
	go func() {
		app := fiber.New(fiber.Config{DisableStartupMessage: true})
		app.Use(pprof.New())

		app.Listen(":8081")
	}()

	router := gin.Default()
	router.Use(middleware.AddOrchestrator(orc))

	v1 := router.Group("/api/v1")
	{
		v1.POST("/azure2aws", runAzure2Aws)
		v1.POST("/awsmapping", runAwsMapping)
		v1.POST("/aws2k8s", runAws2K8s)
		v1.POST("/capsvc2azure", runCapSvc2Azure)
		v1.GET("/mgmt/shutdown", func(c *gin.Context) {
			go func() {
				time.Sleep(time.Second * 2)
				stop()
			}()
			c.String(200, "")
		})
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/metrics", metricsHandler())

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// HTTP server
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Blocks until Context (ctx) is cancelled
	<-ctx.Done()

	if err := srv.Shutdown(ctx); err != nil {
		util.Logger.Info("HTTP Server was unable to shut down gracefully", zap.Error(err))
	}

	util.Logger.Info("Server shutting down")

	backgroundJobWg.Wait()
	util.Logger.Info("All background jobs stopped")
}
