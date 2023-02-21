package main

import (
	"context"
	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"log"
)

func main() {
	util.InitializeLogger()
	err := handler.AwsMappingHandler(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
}
