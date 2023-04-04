package kafkautil

import (
	"testing"
)

func TestNewDialerSasl(t *testing.T) {
	aConf := AuthConfig{
		Brokers:          nil,
		Mechanism:        "plain",
		MechanismOptions: nil,
		Tls:              false,
	}

	dialer, err := NewDialer(aConf)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if dialer == nil {
		t.Error("Dialer is nil, but it is not expected to be")
		t.FailNow()
	}

	if dialer.SASLMechanism == nil {
		t.Error("SASLMechanism is not set when it is expected to be")
		t.FailNow()
	}

	if dialer.SASLMechanism.Name() != "PLAIN" {
		t.Error("SASLMechanism is not set to 'PLAIN' when it is expected to be")
		t.FailNow()
	}
}

func TestNewDialerTls(t *testing.T) {
	aConf := AuthConfig{
		Brokers:          nil,
		Mechanism:        "plain",
		MechanismOptions: nil,
		Tls:              false,
	}

	dialer, err := NewDialer(aConf)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if dialer.TLS != nil {
		t.Error("TLS is set when it is expected to be")
		t.FailNow()
	}

	aConf = AuthConfig{
		Brokers:          nil,
		Mechanism:        "plain",
		MechanismOptions: nil,
		Tls:              true,
	}

	dialer, err = NewDialer(aConf)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if dialer.TLS == nil {
		t.Error("TLS is not set when it is expected to be")
		t.FailNow()
	}

	if len(dialer.TLS.CipherSuites) < 1 {
		t.Error("Not supporting 1 or more cipher suites for TLS")
		t.FailNow()
	}
}
