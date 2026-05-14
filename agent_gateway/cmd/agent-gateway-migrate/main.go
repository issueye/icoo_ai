package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/database"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/migration"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var input string
	var dataDir string
	var backup bool

	flags := flag.NewFlagSet("agent-gateway-migrate", flag.ContinueOnError)
	flags.StringVar(&input, "input", "", "old management settings JSON export")
	flags.StringVar(&dataDir, "data-dir", "", "gateway data directory")
	flags.BoolVar(&backup, "backup", true, "create a timestamped backup before migration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if input == "" {
		return fmt.Errorf("-input is required")
	}
	if dataDir == "" {
		return fmt.Errorf("-data-dir is required")
	}

	db, err := database.OpenSQLite(dataDir)
	if err != nil {
		return err
	}
	defer database.Close(db)
	if err := database.AutoMigrate(db); err != nil {
		return err
	}

	result, err := migration.MigrateManagementSettingsFile(context.Background(), db, input, backup)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
