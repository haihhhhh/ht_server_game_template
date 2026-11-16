// tablepack 为已成功导出的数据生成完整性清单，不负责调用 Luban。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/haihhhhh/ht_server_frame/tables"
	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
)

func run() error {
	dir := flag.String("dir", "tables/exported", "导出目录")
	version := flag.String("version", "development", "项目数据版本")
	flag.Parse()
	m := tables.Manifest{
		DataVersion:   *version,
		SchemaVersion: gamedata.SchemaVersion,
		CodeVersion:   gamedata.CodeVersion,
		ToolVersion:   "luban-5.1.0",
	}
	files, err := filepath.Glob(filepath.Join(*dir, "*.json"))
	if err != nil {
		return err
	}
	for _, name := range files {
		if filepath.Base(name) == "manifest.json" {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		m.Files = append(m.Files, tables.File{
			Name:   filepath.Base(name),
			SHA256: hex.EncodeToString(sum[:]),
		})
	}
	if len(m.Files) == 0 {
		return fmt.Errorf("no exported tables")
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(*dir, "manifest.json"), append(data, '\n'), 0644)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
