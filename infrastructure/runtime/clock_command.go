package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/haihhhhh/ht_server_frame/gameclock"
)

// clockCommand 由本机进程控制台调用。其他受信管理入口也可以直接调用 Clock.SetOffset。
func clockCommand(clock *gameclock.Clock, args string) error {
	fields := strings.Fields(args)
	if len(fields) > 0 {
		var offset time.Duration
		var err error
		switch {
		case len(fields) == 1 && fields[0] == "reset":
			offset = 0
		case len(fields) == 2 && fields[0] == "offset":
			offset, err = time.ParseDuration(fields[1])
			if err != nil {
				return errors.New("偏移格式示例：24h、-30m 或 0s")
			}
		case len(fields) == 2 && fields[0] == "set":
			target, parseErr := time.Parse(time.RFC3339, fields[1])
			if parseErr != nil {
				return errors.New("目标时间必须带时区，例如 2026-11-03T08:00:00+08:00")
			}
			realNow := time.Now()
			offset = target.Sub(realNow)
			if !realNow.Add(offset).Equal(target) {
				return errors.New("目标时间超出可表示的偏移范围")
			}
		default:
			return errors.New("用法：clock | clock offset <时长> | clock set <RFC3339时间> | clock reset")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err = clock.SetOffset(ctx, offset); err != nil {
			return err
		}
		slog.Info("已调整环境业务时钟", "command", fields[0], "offset", offset.String())
	}
	realNow := time.Now()
	businessNow := clock.At(realNow)
	fmt.Printf("系统时间 %s，业务时间 %s，偏移 %s\n", realNow.Format(time.RFC3339), businessNow.Format(time.RFC3339), businessNow.Sub(realNow))
	return nil
}
