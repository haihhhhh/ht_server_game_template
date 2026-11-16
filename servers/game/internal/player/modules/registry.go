// 【通常无需修改｜启动装配】收集各业务包的组件登记和前置顺序，提供 Feature.Get；新增组件在自己的 restore.go 登记。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package modules

import (
	"fmt"
	"sort"
	"sync"

	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
)

type Definition struct {
	Name  string
	After string
}

type factory struct {
	Definition
	slot    int
	restore func(*PlayerComponents, model.Role, *LoadData) (Module, error)
}

var factories []factory

// Feature 是业务包登记时得到的类型化引用，不按字符串查找其他组件。
type Feature[T Module] struct {
	slot int
	name string
}

func (feature Feature[T]) Get(components *PlayerComponents) (T, error) {
	if feature.name != "" && components != nil && feature.slot < len(components.instances) {
		if module, ok := components.instances[feature.slot].(T); ok {
			return module, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("必需玩家组件 %q 尚未装配，请检查前置顺序", feature.name)
}

// Register 在 Go 包初始化时登记一次恢复方法。装配成功的组件自动参与 Tick、下线和保存。
func Register[T interface {
	Module
	comparable
}](definition Definition, restore func(*PlayerComponents, model.Role, *LoadData) (T, error)) Feature[T] {
	slot := len(factories)
	factories = append(factories, factory{
		Definition: definition,
		slot:       slot,
		restore: func(components *PlayerComponents, role model.Role, data *LoadData) (Module, error) {
			result, err := restore(components, role, data)
			var zero T
			if err != nil {
				return nil, err
			}
			if result == zero {
				return nil, fmt.Errorf("必需玩家组件 %s 恢复后为空", definition.Name)
			}
			return result, nil
		},
	})
	return Feature[T]{slot: slot, name: definition.Name}
}

var ordered = sync.OnceValues(func() ([]factory, error) {
	pending := append([]factory(nil), factories...)
	sort.Slice(pending, func(i, j int) bool { return pending[i].Name < pending[j].Name })
	seen := map[string]bool{"": true}
	var result []factory
	for len(pending) > 0 {
		next := -1
		for i, f := range pending {
			if seen[f.Name] {
				return nil, fmt.Errorf("重复的玩家组件 %s", f.Name)
			}
			if seen[f.After] {
				next = i
				break
			}
		}
		if next < 0 {
			return nil, fmt.Errorf("玩家组件 %s 的前置组件 %s 缺失或存在循环依赖", pending[0].Name, pending[0].After)
		}
		f := pending[next]
		seen[f.Name] = true
		result = append(result, f)
		pending = append(pending[:next], pending[next+1:]...)
	}
	return result, nil
})
