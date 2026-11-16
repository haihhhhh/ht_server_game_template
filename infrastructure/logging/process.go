package logging

import (
	"log/slog"
	"os"
	"runtime/debug"
)

// Run 在读取配置之前提供日志；返回退出码，确保清理函数先于 os.Exit 执行。
func Run(service string, run func() error) (code int) {
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("service", service, "instance", "starting"))
	defer slog.SetDefault(previous)
	defer func() {
		if recover() != nil {
			Panic("main")
			code = 1
		}
	}()
	if err := run(); err != nil {
		slog.Error("服务退出", "component", "main", "error", err)
		return 1
	}
	return 0
}

// Panic 只记录位置，不记录可能包含凭据或业务载荷的 panic 值。
func Panic(component string) {
	slog.Error("执行发生异常", "component", component, "kind", "panic", "stack", string(debug.Stack()))
}
