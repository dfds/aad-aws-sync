package kafkautil

import (
	"fmt"
	"testing"
)

func TestConfigResolver(t *testing.T) {
	var conf AuthConfig
	//err := envconfig.Process("AAS_KAFKA", &conf)
	//if err != nil {
	//	log.Fatal("Failed to process auth configurations", zap.Error(err))
	//}
	
	fmt.Printf("%+v\n", conf)
}
