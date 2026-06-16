package observability

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger returns a JSON slog logger at the given level ("debug"..."error").
// When logFile is non-empty, logs are written to both stdout and that file;
// the returned closer releases the file (call it on shutdown).
// NewLogger 按给定级别返回 JSON 格式的 slog logger。
// 当 logFile 非空时，日志同时写到 stdout 和该文件；返回的 closer 在关闭时释放文件。
func NewLogger(level, logFile string) (*slog.Logger, func() error, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}

	var w io.Writer = os.Stdout
	closer := func() error { return nil }
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, err
		}
		// Tee to stdout (for `docker logs`) and the mounted file.
		// 同时写 stdout（便于 docker logs）与挂载的文件。
		w = io.MultiWriter(os.Stdout, f)
		closer = f.Close
	}

	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	return slog.New(h), closer, nil
}
