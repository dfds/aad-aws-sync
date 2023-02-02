package main

import (
	"context"
	"log"

	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
const CAPABILITY_GROUP_PREFIX = "CI_SSU_Cap -"

func main() {
	util.InitializeLogger()
	err := handler.Capsvc2AadHandler(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
}
