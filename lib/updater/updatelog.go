package updater

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// The update log: what the updater did, for the admin page, also after the
// restart an apply ends with (etherpad-lite's var/update.log). It is kept
// small: past logMaxBytes it rotates to .1 … .logBackups.
const (
	logMaxBytes = 1 << 20
	logBackups  = 3
)

// AppendLine writes one line to the log, creating its directory. Errors are
// swallowed: a read-only disk must not break an update.
func AppendLine(path, line string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line + "\n")
	_ = f.Close()
}

// RotateIfNeeded moves path to path.1 (and .1 to .2, …, dropping the oldest)
// once it is larger than maxBytes.
func RotateIfNeeded(path string, maxBytes int64, backups int) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= maxBytes || backups < 1 {
		return
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", path, backups))
	for i := backups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
	}
	_ = os.Rename(path, path+".1")
}

// TailLines returns the last n lines of the log (without newlines); none
// for a missing or empty file.
func TailLines(path string, n int) []string {
	if n <= 0 {
		return []string{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	b = bytes.TrimRight(b, "\n")
	if len(b) == 0 {
		return []string{}
	}
	lines := bytes.Split(b, []byte("\n"))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(bytes.TrimRight(l, "\r"))
	}
	return out
}

// logFile is a zap sink that appends to the update log and rotates it.
type logFile struct {
	path string
	mu   sync.Mutex
}

func (l *logFile) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	RotateIfNeeded(l.path, logMaxBytes, logBackups)
	AppendLine(l.path, string(bytes.TrimRight(p, "\n")))
	return len(p), nil
}

func (l *logFile) Sync() error { return nil }

// withUpdateLog returns logger writing to the update log as well (info and
// up), so every updater message also ends up on the admin page.
func withUpdateLog(logger *zap.SugaredLogger, path string) *zap.SugaredLogger {
	enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey: "t", LevelKey: "l", MessageKey: "m",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	})
	file := zapcore.NewCore(enc, &logFile{path: path}, zap.InfoLevel)
	return logger.Desugar().WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, file)
	})).Sugar()
}
