package logging

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// rotatingWriter 只支持一个进程独占该文件；固定备份数量限制总占用。
type rotatingWriter struct {
	mu          sync.Mutex
	file        *os.File
	path        string
	size, limit int64
	backups     int
}

func openRotating(path string, limit int64, backups int) (*rotatingWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	w := &rotatingWriter{
		path:    path,
		limit:   limit,
		backups: backups,
	}
	err := w.open()
	return w, err
}
func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.file = f
	w.size = info.Size()
	return nil
}
func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if int64(len(p)) > w.limit {
		return 0, errors.New("logging: record exceeds file limit")
	}
	if w.size+int64(len(p)) > w.limit {
		if err := w.file.Close(); err != nil {
			return 0, err
		}
		w.file = nil
		if err := os.Remove(fmt.Sprintf("%s.%d", w.path, w.backups)); err != nil && !os.IsNotExist(err) {
			return 0, err
		}
		for i := w.backups - 1; i >= 1; i-- {
			if err := os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1)); err != nil && !os.IsNotExist(err) {
				return 0, err
			}
		}
		if err := os.Rename(w.path, w.path+".1"); err != nil {
			return 0, err
		}
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}
func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
