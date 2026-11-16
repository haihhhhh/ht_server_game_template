package runtime

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
)

// 管理示例只读取本机标准输入；操作系统账号承担访问控制，不开放远程管理端口。
func console(application *app.App, resources *Resources) error {
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	lines := make(chan string)
	go func() {
		defer close(lines)
		defer func() {
			if recover() != nil {
				logging.Panic("console")
				cancel()
			}
		}()
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 1024), 4096)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-stop.Done():
				return
			}
		}
	}()
	fmt.Println("命令：status | clock [offset <时长> | set <RFC3339时间> | reset] | reload-tables <完整配置包目录> | stop；正常停服使用 stop")
	for {
		select {
		case <-application.Done():
			return stopServer(application)
		case <-stop.Done():
			return stopServer(application)
		case line, ok := <-lines:
			if !ok {
				lines = nil
				continue
			}
			command, arg, _ := strings.Cut(strings.TrimSpace(line), " ")
			switch command {
			case "clock":
				if err := clockCommand(resources.Clock, arg); err != nil {
					fmt.Println(err)
				}
			case "status":
				fmt.Printf("%+v\n", application.Status())
			case "stop":
				return stopServer(application)
			case "reload-tables":
				if resources.Data == nil {
					fmt.Println("当前服务未加载配置表")
					continue
				}
				manifest, err := gamedata.Manifest(strings.TrimSpace(arg))
				if err == nil {
					ctx, finish := context.WithTimeout(context.Background(), 10*time.Second)
					err = resources.Data.Reload(ctx, os.DirFS(strings.TrimSpace(arg)), manifest)
					finish()
				}
				if err != nil {
					fmt.Println("重载失败，保留原配置；检查路径、清单和内容")
				} else {
					fmt.Println("重载完成；业务运行对象由项目决定是否更新")
				}
			default:
				fmt.Println("未知命令")
			}
		}
	}
}
func stopServer(application *app.App) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return application.Stop(ctx)
}
