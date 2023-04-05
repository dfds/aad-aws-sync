package main

import (
	"context"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/event"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	util.InitializeLogger()
	conf, err := config.LoadConfig()
	if err != nil {
		util.Logger.Fatal("Unable to load config", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backgroundJobWg := &sync.WaitGroup{}

	err = event.StartEventHandlers(ctx, conf, backgroundJobWg)
	if err != nil {
		util.Logger.Fatal("", zap.Error(err))
	}
}
