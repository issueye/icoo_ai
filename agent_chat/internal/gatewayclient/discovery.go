package gatewayclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const endpointFileName = "endpoint.json"

func DefaultDataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "icoo-ai", "gateway"), nil
}

func Discover() (Endpoint, string, error) {
	dir, err := DefaultDataDir()
	if err != nil {
		return Endpoint{}, "", err
	}
	return DiscoverFromPath(dir)
}

func DiscoverFromPath(path string) (Endpoint, string, error) {
	endpointPath := path
	if endpointPath == "" {
		dir, err := DefaultDataDir()
		if err != nil {
			return Endpoint{}, "", err
		}
		endpointPath = dir
	}
	info, err := os.Stat(endpointPath)
	if err != nil {
		return Endpoint{}, "", fmt.Errorf("stat gateway endpoint path: %w", err)
	}
	if info.IsDir() {
		endpointPath = filepath.Join(endpointPath, endpointFileName)
	}

	endpoint, err := ReadEndpoint(endpointPath)
	if err != nil {
		return Endpoint{}, "", err
	}
	token, err := ReadToken(endpoint, filepath.Dir(endpointPath))
	if err != nil {
		return Endpoint{}, "", err
	}
	return endpoint, token, nil
}

func ReadEndpoint(path string) (Endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Endpoint{}, fmt.Errorf("read gateway endpoint: %w", err)
	}
	var endpoint Endpoint
	if err := json.Unmarshal(data, &endpoint); err != nil {
		return Endpoint{}, fmt.Errorf("decode gateway endpoint: %w", err)
	}
	if endpoint.BaseURL == "" {
		return Endpoint{}, fmt.Errorf("gateway endpoint baseUrl is empty")
	}
	return endpoint, nil
}

func ReadToken(endpoint Endpoint, fallbackDir string) (string, error) {
	tokenPath := endpoint.TokenFile
	if tokenPath == "" {
		tokenPath = filepath.Join(fallbackDir, "token")
	}
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("read gateway token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func DiscoverClient(path string) (*Client, Endpoint, error) {
	endpoint, token, err := DiscoverFromPath(path)
	if err != nil {
		return nil, Endpoint{}, err
	}
	return New(endpoint.BaseURL, token), endpoint, nil
}
