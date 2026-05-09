package acp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentPackageDoesNotImportACPSDK(t *testing.T) {
	root := filepath.Join("..", "..", "agent")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), "github.com/coder/acp-go-sdk") {
			t.Fatalf("%s imports ACP SDK", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
}
