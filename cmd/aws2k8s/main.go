package main

import (
	"context"
	"go.dfds.cloud/aad-aws-sync/internal/handler"
	"log"
)

func main() {
	err := handler.Aws2K8sHandler(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
}
