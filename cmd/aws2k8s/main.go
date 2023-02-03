package main

import (
	"context"
	"log"

	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"go.dfds.cloud/aad-aws-sync/internal/util"
)

func main() {
	util.InitializeLogger()
	err := handler.Aws2K8sHandler(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
}
