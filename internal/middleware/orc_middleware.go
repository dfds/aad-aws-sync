package middleware

import (
	"github.com/gin-gonic/gin"
	"go.dfds.cloud/orchestrator"
)

func AddOrchestrator(orc *orchestrator.Orchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Keys = map[string]any{}
		c.Keys["orchestrator"] = orc
	}
}

func GetOrchestrator(c *gin.Context) *orchestrator.Orchestrator {
	orcA, _ := c.Get("orchestrator")
	o := orcA.(*orchestrator.Orchestrator)
	return o
}
