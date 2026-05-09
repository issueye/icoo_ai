package config

import (
	"errors"
	"net"
	"strings"
)

const Version = "0.1.0-dev"

type Config struct {
	Host    string
	Port    int
	DataDir string
	Version string
}

func Default() Config {
	return Config{
		Host:    "127.0.0.1",
		Port:    0,
		Version: Version,
	}
}

func (c Config) Validate() error {
	host := strings.TrimSpace(c.Host)
	if host == "" {
		return errors.New("host is required")
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return errors.New("gateway host must be a loopback address")
	}
	if c.Port < 0 || c.Port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	return nil
}
