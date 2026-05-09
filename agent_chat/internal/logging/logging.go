package logging

import (
	"bufio"
	"fmt"
	"io"
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
	writer, resolvedLogFilePath := buildLogWriter(cfg.FilePath)

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	logger := slog.New(handler).With("app", app)
	slog.SetDefault(logger)
	logger.Info("logging configured", "level", cfg.Level, "format", format, "logFilePath", resolvedLogFilePath)
	return logger
}

type Config struct {
	Level    string
	Format   string
	FilePath string
}

func defaultConfig() Config {
	return Config{
		Level:    "info",
		Format:   "text",
		FilePath: "logs/agent_chat.log",
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
		case "log_file_path":
			parsed, err := strconv.Unquote(value)
			if err == nil && strings.TrimSpace(parsed) != "" {
				cfg.FilePath = strings.TrimSpace(parsed)
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

func buildLogWriter(rawPath string) (io.Writer, string) {
	path := resolveLogFilePath(rawPath)
	if path == "" {
		return os.Stderr, ""
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "agent_chat logging create log directory failed: %v\n", err)
		return os.Stderr, ""
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent_chat logging open log file failed: %v\n", err)
		return os.Stderr, ""
	}
	return io.MultiWriter(os.Stderr, file), path
}

func resolveLogFilePath(rawPath string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	wd, err := os.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return filepath.Clean(trimmed)
	}
	return filepath.Join(wd, trimmed)
}
