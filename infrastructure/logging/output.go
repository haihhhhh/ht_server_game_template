package logging

import (
	"errors"
	"io"
	"time"
)

// outputWriter 分别尝试各个输出。每个实例由同一个 slog 处理器串行写入。
type outputWriter struct {
	file        io.Writer
	console     bool
	stderr      io.Writer
	lastWarning time.Time
}

func (w *outputWriter) Write(p []byte) (int, error) {
	var failures []error
	write := func(dst io.Writer) error {
		n, err := dst.Write(p)
		if err == nil && n != len(p) {
			err = io.ErrShortWrite
		}
		return err
	}
	if w.console {
		if err := write(w.stderr); err != nil {
			failures = append(failures, err)
		}
	}
	if w.file != nil {
		if err := write(w.file); err != nil {
			failures = append(failures, err)
			if !w.console {
				if fallbackErr := write(w.stderr); fallbackErr != nil {
					failures = append(failures, fallbackErr)
				}
			}
		}
	}
	if len(failures) > 0 {
		now := time.Now()
		if w.lastWarning.IsZero() || now.Sub(w.lastWarning) >= time.Minute {
			w.lastWarning = now
			_, _ = io.WriteString(w.stderr, "日志输出失败，请检查磁盘、权限和文件占用；文件失败时日志转至标准错误输出。\n")
		}
		return 0, errors.Join(failures...)
	}
	return len(p), nil
}
