package kafkautil

import (
	"testing"
)

func TestNewProducer(t *testing.T) {
	authConfig := AuthConfig{
		Brokers:          []string{"localhost:9092"},
		Mechanism:        "plain",
		MechanismOptions: nil,
		Tls:              false,
	}

	dialer, err := NewDialer(authConfig)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	writer := NewProducer(
		ProducerConfig{Topic: ""},
		authConfig,
		dialer,
	)

	if writer == nil {
		t.Error("Instance of Writer not returned as expected")
		t.FailNow()
	}
}
