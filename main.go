package main

import (
	"go.dfds.cloud/aad-aws-sync/util"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	util.InitializeLogger()
	defer util.Logger.Sync()

	util.Logger.Info("Initialising Orchestrator")
	orc := util.NewOrchestrator()
	orc.Init()
}
