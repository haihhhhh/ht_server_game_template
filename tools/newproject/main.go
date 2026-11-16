// newproject 从模板创建平级的服务端、协议和表格仓库。请在模板根目录运行。
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type project struct {
	source string
	target string
}

func command(dir, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd
}
func run(dir, name string, args ...string) error {
	cmd := command(dir, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// 直接执行仓库已有的生成脚本；模块创建工具不复刻协议或配置表的生成规则。
func script(dir, name string) error {
	if runtime.GOOS == "windows" {
		return run(dir, "cmd", "/d", "/c", filepath.Join("scripts", name+".bat"))
	}
	return run(dir, "sh", filepath.Join("scripts", name+".sh"))
}

func create(module string) error {
	valid := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9][A-Za-z0-9._-]*)+$`)
	if !valid.MatchString(module) || strings.Contains(module, "..") {
		return errors.New("请输入完整 Go module 名称，例如 git.example.com/game/mygame")
	}
	source, err := os.Getwd()
	if err != nil {
		return err
	}
	sourceFile := filepath.Join(source, "go.mod")
	if _, err = os.Stat(sourceFile); err != nil {
		return errors.New("请在模板根目录执行")
	}
	goExe := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goExe += ".exe"
	}
	if err = os.Setenv("GO_EXE", goExe); err != nil {
		return err
	}
	if err = os.Setenv("GOWORK", "off"); err != nil {
		return err
	}
	value, err := command(source, goExe, "list", "-m").Output()
	if err != nil {
		return fmt.Errorf("读取模板 module: %w", err)
	}
	templateModule := strings.TrimSpace(string(value))
	if module == templateModule {
		return errors.New("新项目的 module 不能与模板相同")
	}
	sourceName := path.Base(templateModule)
	sourcePrefix := strings.TrimSuffix(sourceName, "_template")
	projectName := path.Base(module)
	if regexp.MustCompile(`^v[2-9][0-9]*$|^v1[0-9]+$`).MatchString(projectName) {
		projectName = path.Base(path.Dir(module))
	}
	if !filepath.IsLocal(projectName) {
		return errors.New("无效的项目目录名")
	}
	parent := filepath.Dir(source)
	projects := []project{
		{source, filepath.Join(parent, projectName)},
		{filepath.Join(parent, sourcePrefix+"_protocol"), filepath.Join(parent, projectName+"_protocol")},
		{filepath.Join(parent, sourcePrefix+"_tables"), filepath.Join(parent, projectName+"_tables")},
	}
	for _, repo := range projects {
		info, statErr := os.Stat(repo.source)
		if statErr != nil || !info.IsDir() {
			return fmt.Errorf("缺少源目录 %s", repo.source)
		}
		if _, statErr = os.Lstat(repo.target); !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("目标目录已存在或无法访问：%s", repo.target)
		}
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	tools := []string{"protoc" + suffix, "protoc-gen-go" + suffix}
	for _, tool := range tools {
		if _, err = os.Stat(filepath.Join(source, ".tools", "protobuf", "bin", tool)); err != nil {
			return errors.New("模板尚未准备协议工具，请先运行 Install-ProtocolTools")
		}
	}
	replace := strings.NewReplacer(templateModule, module, sourcePrefix+"_protocol", projectName+"_protocol", sourcePrefix+"_tables", projectName+"_tables", sourceName, projectName)
	for _, repo := range projects {
		if err = os.Mkdir(repo.target, 0755); err != nil {
			return err
		}
		fmt.Println("创建", repo.target)
		if err = run(repo.target, "git", "init", "-q"); err != nil {
			return err
		}
		if err = copySources(repo, replace); err != nil {
			return fmt.Errorf("复制源码到 %s 未完成，请检查已创建的目录后重新创建: %w", repo.target, err)
		}
	}
	target := projects[0].target
	for _, tool := range tools {
		rel := filepath.Join(".tools", "protobuf", "bin", tool)
		if err = copyFile(source, target, rel, nil); err != nil {
			return err
		}
	}
	if err = script(target, "Generate-Protocol"); err != nil {
		return fmt.Errorf("协议生成失败，可在 %s 重新运行 Generate-Protocol: %w", target, err)
	}
	if err = script(target, "Generate-Tables"); err != nil {
		return fmt.Errorf("表格生成失败，可在 %s 重新运行 Generate-Tables: %w", target, err)
	}
	if err = checkReferences(projects, templateModule, module); err != nil {
		return err
	}
	if err = script(target, "Build"); err != nil {
		return fmt.Errorf("构建失败，可在 %s 重新运行 Build: %w", target, err)
	}
	fmt.Printf("项目已创建并通过编译：%s\n", target)
	return nil
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "用法：Create-Project <新项目的 Go module 名称>")
		os.Exit(1)
	}
	if err := create(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
