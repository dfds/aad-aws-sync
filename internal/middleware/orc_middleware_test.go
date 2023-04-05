package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.dfds.cloud/aad-aws-sync/internal/orchestrator"
	"sync"
	"testing"
)

func TestAddOrchestrator(t *testing.T) {
	backgroundJobWg := &sync.WaitGroup{}
	orc := orchestrator.NewOrchestrator(context.Background(), backgroundJobWg)

	f := AddOrchestrator(orc)
	assert.NotNil(t, f)

	gCtx := &gin.Context{}
	f(gCtx)

	assert.NotNil(t, gCtx.Keys["orchestrator"])
}

func TestGetOrchestrator(t *testing.T) {
	backgroundJobWg := &sync.WaitGroup{}
	orc := orchestrator.NewOrchestrator(context.Background(), backgroundJobWg)

	f := AddOrchestrator(orc)
	assert.NotNil(t, f)

	gCtx := &gin.Context{}
	f(gCtx)
	assert.NotNil(t, gCtx.Keys["orchestrator"])

	gOrc := GetOrchestrator(gCtx)
	assert.NotNil(t, gOrc)
}
