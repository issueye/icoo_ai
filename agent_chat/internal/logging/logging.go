package logging

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ConfigureDefault(app string) *slog.Logger {
	cfg := loadConfig()
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}
	format := strings.ToLower(strings.TrimSpace(cfg.Format))

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler).With("app", app)
	slog.SetDefault(logger)
	return logger
}

type Config struct {
	Level  string
	Format string
}

func defaultConfig() Config {
	return Config{
		Level:  "info",
		Format: "text",
	}
}

func settingsFilePath() string {
	wd, err := os.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return "chat.toml"
	}
	return filepath.Join(wd, "chat.toml")
}

func loadConfig() Config {
	cfg := defaultConfig()
	path := settingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
			if line == "" {
				continue
			}
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "log_level":
			parsed, err := strconv.Unquote(value)
			if err == nil && strings.TrimSpace(parsed) != "" {
				cfg.Level = strings.TrimSpace(parsed)
			}
		case "log_format":
			parsed, err := strconv.Unquote(value)
			if err == nil && strings.TrimSpace(parsed) != "" {
				cfg.Format = strings.TrimSpace(parsed)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "agent_chat logging config scan failed: %v\n", err)
	}
	return cfg
}

func parseLevel(raw string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
