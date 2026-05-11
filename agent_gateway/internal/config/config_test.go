package config

import "testing"

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := Default()
	cfg.DataDir = "./.agent_gateway"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsNonLoopbackHost(t *testing.T) {
	cfg := Default()
	cfg.DataDir = "./.agent_gateway"
	cfg.Host = "0.0.0.0"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want non-loopback validation error")
	}
}
