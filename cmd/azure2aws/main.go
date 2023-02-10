package main

import (
	"context"
	"log"

	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

func main() {
	util.InitializeLogger()
	err := handler.Azure2AwsHandler(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
}
