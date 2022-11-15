package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.dfds.cloud/aad-aws-sync/util"
	"net/http"
	"time"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	util.InitializeLogger()
	defer util.Logger.Sync()
	http.Handle("/metrics", promhttp.Handler())

	go func() {
		time.Sleep(time.Second * 2)
		util.Logger.Info("Initialising Orchestrator")
		orc := util.NewOrchestrator()
		orc.Init()
	}()

	http.ListenAndServe(":8080", nil)
}
