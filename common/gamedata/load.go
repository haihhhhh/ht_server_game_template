// TODO(业务扩展)【配置表】新增表的校验与纯派生数据在此接入；切换后的在线对象如何变化，由各业务组件决定。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package gamedata

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/haihhhhh/ht_server_frame/tables"
	generated "github.com/haihhhhh/ht_server_game_template/common/gamedata/generated"
)

// Data 是一份配置的只读入口。原表与等级索引同时发布，业务只能通过查询方法取值。
type Data struct {
	raw    *generated.Tables
	levels []*generated.LevelEntry
}

func (data Data) Text(id string) (string, bool) {
	row := data.raw.TbText.Get(id)
	if row == nil {
		return "", false
	}
	return row.Text, true
}

const SchemaVersion = "gameplay-v2"
const CodeVersion = "gameplay-reader-v2"

func Manifest(dir string) (tables.Manifest, error) {
	var m tables.Manifest
	f, err := os.Open(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return m, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return m, err
	}
	if info.Size() > 65536 {
		return m, errors.New("table manifest too large")
	}
	err = json.NewDecoder(f).Decode(&m)
	return m, err
}

func New(dir string) (*tables.Module[Data], error) {
	m, err := Manifest(dir)
	if err != nil {
		return nil, err
	}
	return tables.New(os.DirFS(dir), m, tables.Options[Data]{
		SchemaVersion: SchemaVersion,
		CodeVersion:   CodeVersion,
		RequiredFiles: []string{"tbtext.json", "tbitem.json", "tblevel.json"},
		MaxFiles:      32,
		MaxFileBytes:  1 << 20,
		MaxTotalBytes: 4 << 20,
		Decode: func(ctx context.Context, files map[string][]byte) (Data, error) {
			loaded, err := generated.NewTables(func(name string) ([]map[string]interface{}, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				var rows []map[string]interface{}
				err := json.Unmarshal(files[name+".json"], &rows)
				return rows, err
			})
			if err != nil {
				return Data{}, err
			}
			if len(loaded.TbText.GetDataList()) != len(loaded.TbText.GetDataMap()) {
				return Data{}, errors.New("duplicate text ID")
			}
			if len(loaded.TbItem.GetDataList()) != len(loaded.TbItem.GetDataMap()) {
				return Data{}, errors.New("duplicate item ID")
			}
			return Data{raw: loaded, levels: sortedLevels(loaded.TbLevel)}, nil
		},
		Validate: func(_ context.Context, data Data) error {
			if welcome, _ := data.Text("welcome"); welcome == "" {
				return errors.New("welcome text missing")
			}
			return data.ValidateGameplay()
		},
		// Data 只暴露只读查询。复制入口即可共享整版配置。
		Clone: func(data Data) Data { return data },
	})
}
