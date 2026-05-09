package bridge

import "log/slog"

var logger = slog.Default().With("component", "bridge")
