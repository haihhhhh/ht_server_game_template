package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Git 克隆保留已跟踪文件和未忽略的新源码。下载 ZIP 时借用新仓库的空索引读取源目录的 .gitignore。
func copySources(repo project, replace *strings.Replacer) error {
	args := []string{"-c", "safe.directory=" + filepath.ToSlash(repo.source)}
	if _, err := os.Lstat(filepath.Join(repo.source, ".git")); errors.Is(err, os.ErrNotExist) {
		args = append(args, "--git-dir="+filepath.Join(repo.target, ".git"), "--work-tree="+repo.source)
	}
	args = append(args, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd := command(repo.source, "git", args...)
	cmd.Stderr = os.Stderr
	listing, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("读取 %s 的源码清单: %w", repo.source, err)
	}
	for _, name := range strings.Split(string(listing), "\x00") {
		if name == "" {
			continue
		}
		rel := filepath.FromSlash(name)
		if !filepath.IsLocal(rel) {
			return fmt.Errorf("无效源路径 %s", rel)
		}
		err = copyFile(repo.source, repo.target, rel, replace)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, target, rel string, replace *strings.Replacer) error {
	src := filepath.Join(source, rel)
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("仅支持普通源文件：%s", src)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if replace != nil && utf8.Valid(data) && bytes.IndexByte(data, 0) < 0 {
		data = []byte(replace.Replace(string(data)))
	}
	dst := filepath.Join(target, rel)
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

// 源文件和新生成的文本都要检查，避免后续生成重新带回模板路径。
func checkReferences(projects []project, oldModule, newModule string) error {
	for _, repo := range projects {
		err := filepath.WalkDir(repo.target, func(file string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == ".git" || entry.Name() == ".tools" || file == filepath.Join(repo.target, "bin") {
					return filepath.SkipDir
				}
				return nil
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			if utf8.Valid(data) && bytes.IndexByte(data, 0) < 0 {
				content := string(data)
				if strings.Contains(newModule, oldModule) {
					content = strings.ReplaceAll(content, newModule, "")
				}
				if strings.Contains(content, oldModule) {
					return fmt.Errorf("文件仍引用旧 module：%s", file)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
