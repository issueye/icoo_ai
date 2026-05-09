package config

import "testing"

func TestConfigValidateRequiresLoopbackHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{name: "localhost ipv4", host: "127.0.0.1"},
		{name: "localhost ipv6", host: "::1"},
		{name: "public host", host: "0.0.0.0", wantErr: true},
		{name: "dns name", host: "localhost", wantErr: true},
		{name: "blank", host: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Host = tt.host
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestConfigValidateRejectsInvalidPort(t *testing.T) {
	cfg := Default()
	cfg.Port = 70000
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid port error")
	}
}

func TestConfigValidateRequiresACPCommandWhenEnabled(t *testing.T) {
	cfg := Default()
	cfg.ACP.Enabled = true
	cfg.ACP.Command = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want acp command error")
	}
}
