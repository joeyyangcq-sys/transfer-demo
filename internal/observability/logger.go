package observability

import (
	"log/slog"
	"os"
)

// NewLogger returns a JSON slog logger at the given level ("debug"..."error").
// NewLogger 按给定级别（"debug"..."error"）返回一个 JSON 格式的 slog logger。
func NewLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}
