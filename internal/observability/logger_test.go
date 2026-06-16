package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewLogger_WritesErrorToFile checks that error logs land in the mounted
// log file in JSON form.
// TestNewLogger_WritesErrorToFile 校验错误日志以 JSON 形式写入挂载的日志文件。
func TestNewLogger_WritesErrorToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")

	log, closeLog, err := NewLogger("info", path)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	log.Error("request failed", "type", "internal", "layer", "service")
	if err := closeLog(); err != nil {
		t.Fatalf("close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"level":"ERROR"`) || !strings.Contains(content, `"msg":"request failed"`) {
		t.Errorf("log file missing error entry, got: %s", content)
	}
}

// TestNewLogger_Stdout covers the no-file path and an invalid level fallback.
// TestNewLogger_Stdout 覆盖无文件路径与非法级别回退。
func TestNewLogger_Stdout(t *testing.T) {
	log, closeLog, err := NewLogger("not-a-level", "")
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	if log == nil {
		t.Fatal("nil logger")
	}
	if err := closeLog(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestNewLogger_BadPath returns an error when the file cannot be opened.
// TestNewLogger_BadPath 在文件无法打开时返回错误。
func TestNewLogger_BadPath(t *testing.T) {
	if _, _, err := NewLogger("info", filepath.Join(t.TempDir(), "nope", "app.log")); err == nil {
		t.Error("expected an error for an unwritable path")
	}
}
