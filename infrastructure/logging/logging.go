package logging

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level   string `json:"level"`
	Format  string `json:"format"`
	Source  bool   `json:"source"`
	Console bool   `json:"console"`
	File    string `json:"file"`
	MaxMB   int    `json:"max_mb"`
	Backups int    `json:"backups"`
}

func Default() Config {
	return Config{
		Level:   "info",
		Format:  "json",
		Console: true,
		MaxMB:   20,
		Backups: 3,
	}
}
func (c Config) Validate() error {
	var level slog.Level
	if level.UnmarshalText([]byte(c.Level)) != nil || (c.Format != "json" && c.Format != "text") || (!c.Console && c.File == "") {
		return errors.New("logging: invalid output or level")
	}
	if c.MaxMB < 1 || c.MaxMB > 1024 || c.Backups < 1 || c.Backups > 20 {
		return errors.New("logging: invalid rotation limits")
	}
	return nil
}
func Open(c Config, service, instance string) (*slog.Logger, io.Closer, error) {
	if err := c.Validate(); err != nil {
		return nil, nil, err
	}
	writer := &outputWriter{
		console: c.Console,
		stderr:  os.Stderr,
	}
	var closer io.Closer = io.NopCloser(strings.NewReader(""))
	if c.File != "" {
		w, err := openRotating(c.File, int64(c.MaxMB)<<20, c.Backups)
		if err != nil {
			return nil, nil, err
		}
		writer.file = w
		closer = w
	}
	var level slog.Level
	_ = level.UnmarshalText([]byte(c.Level))
	options := &slog.HandlerOptions{
		Level:     level,
		AddSource: c.Source,
	}
	var h slog.Handler = slog.NewJSONHandler(writer, options)
	if c.Format == "text" {
		h = slog.NewTextHandler(writer, options)
	}
	return slog.New(h).With("service", service, "instance", instance), closer, nil
}
