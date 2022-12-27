package config

import (
	"encoding/json"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	conf, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	serialised, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(serialised))
}
