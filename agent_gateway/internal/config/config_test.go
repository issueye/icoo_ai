package config

import "testing"

func TestDefaultConfigACPPoolSize(t *testing.T) {
	cfg := Default()
	if cfg.ACP.PoolSize != 1 {
		t.Fatalf("cfg.ACP.PoolSize = %d, want 1", cfg.ACP.PoolSize)
	}
}

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

func TestConfigValidateRejectsInvalidACPPoolSize(t *testing.T) {
	tests := []struct {
		name     string
		poolSize int
		wantErr  bool
	}{
		{name: "zero", poolSize: 0, wantErr: true},
		{name: "negative", poolSize: -1, wantErr: true},
		{name: "positive", poolSize: 2, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.ACP.PoolSize = tt.poolSize
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want invalid acp pool size error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
