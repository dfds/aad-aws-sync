package kafkautil

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"log"
	"testing"
)

func TestConfigResolver(t *testing.T) {
	var conf Config
	err := envconfig.Process("AAS_KAFKA", &conf)
	if err != nil {
		log.Fatal("Failed to process auth configurations", zap.Error(err))
	}

	fmt.Printf("%+v\n", conf)
}
